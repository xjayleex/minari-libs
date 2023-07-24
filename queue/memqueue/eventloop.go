package memqueue

import (
	"time"

	"github.com/xjayleex/minari-libs/logpack"
	"github.com/xjayleex/minari-libs/queue"
)

type eventLoop interface {
	run()
	processACK(chanList, int)
}

type bufferingEventLoop struct {
	broker     *broker
	deleteChan chan int

	// The current buffer that incoming events are appended to. When it gets full enough,
	// or enough time has passed, it is added to flushList
	// Events will still be added to buf even after it is in flushList, until
	// either it reaches minEvents or a consumer requests it.
	buf *batchBuffer

	// flushList is the list of buffers that are ready to be sent to consumers.
	flushList flushList

	// pendingACKs aggregates a list of ACK channels for batches that have been sent
	// to consumers, which is then sent to the broker's scheduledACKs channel.
	pendingACKs chanList

	// The number of events currently waiting in the queue, including
	// those that have not yet been acked.
	eventCount int

	nextConsumedID queue.EntryID
	nextACKedID    queue.EntryID

	minEvents    int
	maxEvents    int
	flushTimeout time.Duration

	// buffer flush timer state
	timer *time.Timer
	idleC <-chan time.Time

	nextEntryID queue.EntryID
}

func newBufferingEventLoop(b *broker, size int, minEvents int, flushTimeout time.Duration) *bufferingEventLoop {
	loop := &bufferingEventLoop{
		broker:       b,
		deleteChan:   make(chan int),
		maxEvents:    size,
		minEvents:    minEvents,
		flushTimeout: flushTimeout,
	}
	loop.buf = newBatchBuffer(loop.minEvents)

	loop.timer = time.NewTimer(flushTimeout)
	if !loop.timer.Stop() {
		<-loop.timer.C
	}
	return loop
}

func (l *bufferingEventLoop) run() {
	broker := l.broker

	for {
		var pushChan chan pushRequest

		// Push channel is enabled if the queue is not yet full
		if l.eventCount < l.maxEvents {
			pushChan = broker.pushChan
		}

		var getChan chan getRequest
		if !l.flushList.empty() {
			getChan = broker.getChan
		}

		var schedACKs chan chanList
		// Enable sending to the scheduled ACKs channel if we have something to send.
		if !l.pendingACKs.empty() {
			schedACKs = broker.scheduledACKs
		}

		select {
		case <-broker.done:
			return
		case req := <-pushChan:
			l.handleInsert(&req)
		case req := <-broker.cancelChan:
			l.handleCancel(&req)
		case req := <-getChan:
			l.handleGetRequest(&req)
		case schedACKs <- l.pendingACKs:
			l.pendingACKs = chanList{}
		case count := <-l.deleteChan:
			l.handleDelete(count)
		case req := <-l.broker.metricChan:
			l.handleMetricsRequest(&req)
		case <-l.idleC:
			l.idleC = nil
			l.timer.Stop()
			if l.buf.length() > 0 {
				l.flushBuffer()
			}
		}
	}
}

func (l *bufferingEventLoop) handleMetricsRequest(req *metricsRequest) {
	req.responseChan <- memQueueMetrics{
		currentQueueSize: l.eventCount,
		occupiedRead:     int(l.nextConsumedID - l.nextACKedID),
		oldestEntryID:    l.nextACKedID,
	}
}

func (l *bufferingEventLoop) handleInsert(req *pushRequest) {
	if l.insert(req, l.nextEntryID) {
		req.resp <- l.nextEntryID

		l.nextEntryID++
		l.eventCount++

		L := l.buf.length()
		if !l.buf.flushed {
			if L < l.minEvents {
				l.startFlushTimer()
			} else {
				l.stopFlushTimer()
				l.flushBuffer()
				l.buf = newBatchBuffer(l.minEvents)
			}
		} else if L >= l.minEvents {
			l.buf = newBatchBuffer(l.minEvents)
		}
	}
}

func (l *bufferingEventLoop) insert(req *pushRequest, id queue.EntryID) bool {
	if req.producer != nil && req.producer.state.canceled {
		reportCanceledState(l.broker.logger, req)
		return false
	}

	l.buf.add(queueEntry{
		event:      req.event,
		id:         id,
		producer:   req.producer,
		producerID: req.producerID,
	})
	return true
}

func (l *bufferingEventLoop) handleCancel(req *producerCancelRequest) {
	var removed int
	if producer := req.producer; producer != nil {
		// remove from actively finished buffers
		for buf := l.flushList.head; buf != nil; buf = buf.next {
			removed += buf.cancel(producer)
		}
		if !l.buf.flushed {
			removed += l.buf.cancel(producer)
		}
		producer.state.canceled = true
	}

	if req.resp != nil {
		req.resp <- producerCancelResponse{removed: removed}
	}

	// remove flushed but empty buffers
	tmpList := flushList{}
	for l.flushList.head != nil {
		b := l.flushList.head
		l.flushList.head = b.next

		if b.length() > 0 {
			tmpList.add(b)
		}
	}
	l.flushList = tmpList
	l.eventCount -= removed
}

func (l *bufferingEventLoop) handleGetRequest(req *getRequest) {
	buf := l.flushList.head
	if buf == nil {
		panic("get from non-flushed buffers")
	}

	count := buf.length()
	if count == 0 {
		panic("empty buffer in flush list")
	}

	if size := req.entryCount; size > 0 && size < count {
		count = size
	}

	if count == 0 {
		panic("empty batch returned")
	}

	entries := buf.entries[:count]
	acker := newBatchACKState(0, count, entries)

	req.responseChan <- getResponse{
		ackChan: acker.doneChan,
		entries: entries,
	}
	l.pendingACKs.append(acker)

	l.nextConsumedID += queue.EntryID(len(entries))
	buf.entries = buf.entries[count:]
	if buf.length() == 0 {
		l.advanceFlushList()
	}
}

func (l *bufferingEventLoop) handleDelete(count int) {
	l.nextACKedID += queue.EntryID(count)
	l.eventCount -= count
}

func (l *bufferingEventLoop) startFlushTimer() {
	if l.idleC == nil {
		l.timer.Reset(l.flushTimeout)
		l.idleC = l.timer.C
	}
}

func (l *bufferingEventLoop) stopFlushTimer() {
	if l.idleC != nil {
		l.idleC = nil
		if !l.timer.Stop() {
			<-l.timer.C
		}
	}
}

func (l *bufferingEventLoop) advanceFlushList() {
	l.flushList.pop()
	if l.flushList.count == 0 && l.buf.flushed {
		l.buf = newBatchBuffer(l.minEvents)
	}
}

func (l *bufferingEventLoop) flushBuffer() {
	l.buf.flushed = true
	l.flushList.add(l.buf)
}

func (l *bufferingEventLoop) processACK(lst chanList, N int) {
	ackCallbacks := []func(){}
	// First we traverse the entries we're about to remove, collecting any callbacks
	// we need to run.
	lst.reverse()
	for !lst.empty() {
		current := lst.pop()
		entries := current.entries

		// Traverse entries from last to first, so we can acknowledge the most recent
		// ones first and skip subsequent producer callbacks.
		for i := len(entries) - 1; i >= 0; i-- {
			entry := &entries[i]
			if entry.producer == nil {
				continue
			}

			if entry.producerID <= entry.producer.state.lastACK {
				// This index was already acknowledged on a previous iteration, skip.
				entry.producer = nil
				continue
			}
			producerState := entry.producer.state
			count := int(entry.producerID - producerState.lastACK)
			ackCallbacks = append(ackCallbacks, func() { producerState.cb(count) })
			entry.producer.state.lastACK = entry.producerID
			entry.producer = nil
		}
	}
	// Signal the queue to delete the events
	l.deleteChan <- N
	// The events have been removed; notify their listeners.
	for _, f := range ackCallbacks {
		f()
	}
}

// flushList
type flushList struct {
	head  *batchBuffer
	tail  *batchBuffer
	count int
}

func (l *flushList) pop() {
	l.count--
	if l.count > 0 {
		l.head = l.head.next
	} else {
		l.head = nil
		l.tail = nil
	}
}

func (l *flushList) empty() bool {
	return l.head == nil
}

func (l *flushList) add(b *batchBuffer) {
	l.count++
	b.next = nil
	if l.tail == nil {
		l.head = b
		l.tail = b
	} else {
		l.tail.next = b
		l.tail = b
	}
}

func reportCanceledState(log logpack.Logger, req *pushRequest) {
	if callback := req.producer.state.dropCB; callback != nil {
		callback(req.event)
	}
}
