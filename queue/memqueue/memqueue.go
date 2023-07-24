package memqueue

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/xjayleex/minari-libs/logpack"
	"github.com/xjayleex/minari-libs/queue"
)

const (
	minInputQueueSize      = 20
	maxInputQueueSizeRatio = 0.1
)

type broker struct {
	done    chan struct{}
	logger  logpack.Logger
	bufSize int

	pushChan   chan pushRequest
	getChan    chan getRequest
	cancelChan chan producerCancelRequest

	scheduledACKs chan chanList
	ackListener   queue.ACKListener
	metricChan    chan metricsRequest

	wg sync.WaitGroup
}

type Setting struct {
	ACKListener    queue.ACKListener
	Events         int
	FlushMinEvents int
	FlushTimeout   time.Duration
	InputQueueSize int
}

type queueEntry struct {
	event interface{}
	id    queue.EntryID

	producer   *ackProducer
	producerID producerID // The order of this entry within its producer
}

type batch struct {
	queue    *broker
	entries  []queueEntry
	doneChan chan batchDoneMsg
}

func (b *batch) Count() int {
	return len(b.entries)
}

func (b *batch) Entry(i int) interface{} {
	return b.entries[i].event
}

func (b *batch) FreeEntries() {
	// memqueue cant release event references til they are fully acked, no-op
}

func (b *batch) Done() {
	// DELETME
	fmt.Println("DELETME : BATCH DONE CALLED")
	b.doneChan <- batchDoneMsg{}
}

type batchACKState struct {
	next         *batchACKState
	doneChan     chan batchDoneMsg
	start, count int // number of events waiting for ACK
	entries      []queueEntry
}

type chanList struct {
	head *batchACKState
	tail *batchACKState
}

func (l *chanList) prepend(ch *batchACKState) {
	ch.next = l.head
	l.head = ch
	if l.tail == nil {
		l.tail = ch
	}
}

func (l *chanList) concat(other *chanList) {
	if other.head == nil {
		return
	}

	if l.head == nil {
		*l = *other
		return
	}

	l.tail.next = other.head
	l.tail = other.tail
}

func (l *chanList) append(ch *batchACKState) {
	if l.head == nil {
		l.head = ch
	} else {
		l.tail.next = ch
	}
	l.tail = ch
}

func (l *chanList) empty() bool {
	return l.head == nil
}

func (l *chanList) front() *batchACKState {
	return l.head
}

func (l *chanList) nextBatchChannel() chan batchDoneMsg {
	if l.head == nil {
		return nil
	}
	return l.head.doneChan
}

func (l *chanList) pop() *batchACKState {
	ch := l.head
	if ch != nil {
		l.head = ch.next
		if l.head == nil {
			l.tail = nil
		}
	}

	ch.next = nil
	return ch
}

func (l *chanList) reverse() {
	origin := *l
	*l = chanList{}

	for !origin.empty() {
		l.prepend(origin.pop())
	}
}

func New(logger logpack.Logger, setting Setting) *broker {
	size := setting.Events
	minEvents := setting.FlushMinEvents
	flushTimeout := setting.FlushTimeout

	chanSize := AdjustInputQueueSize(setting.InputQueueSize, size)

	if minEvents < 1 {
		minEvents = 1
	}
	if minEvents > 1 && flushTimeout <= 0 {
		minEvents = 1
		flushTimeout = 0
	}
	if minEvents > size {
		minEvents = size
	}

	if logger == nil {
		// logger = logpack.NewLogger("memqueue")
		// FIXME :
		logger = logpack.G()
	}

	b := &broker{
		done:          make(chan struct{}),
		logger:        logger,
		bufSize:       size,
		pushChan:      make(chan pushRequest, chanSize),
		getChan:       make(chan getRequest),
		cancelChan:    make(chan producerCancelRequest, 5),
		scheduledACKs: make(chan chanList),
		ackListener:   setting.ACKListener,
		metricChan:    make(chan metricsRequest),
	}

	var eventLoop eventLoop
	if minEvents > 1 {
		eventLoop = newBufferingEventLoop(b, size, minEvents, flushTimeout)
	} else {
		panic("directEventLoop not implemented yet.")
	}

	b.bufSize = size
	ackLoop := &ackLoop{
		broker:     b,
		processACK: eventLoop.processACK,
	}

	b.wg.Add(2)
	go func() {
		defer b.wg.Done()
		eventLoop.run()
	}()
	go func() {
		defer b.wg.Done()
		ackLoop.run()
	}()

	return b
}

func (b *broker) Close() error {
	close(b.done)
	return nil
}

func (b *broker) BufferConfig() queue.BufferConfig {
	return queue.BufferConfig{
		MaxEvents: b.bufSize,
	}
}

func (b *broker) Producer(cfg queue.ProducerConfig) queue.Producer {
	return newProducer(b, cfg.ACK, cfg.OnDrop, cfg.DropOnCancel)
}

func (b *broker) Get(count int) (queue.Batch, error) {
	responseChan := make(chan getResponse, 1)
	select {
	case <-b.done:
		return nil, io.EOF
	case b.getChan <- getRequest{entryCount: count, responseChan: responseChan}:
	}

	resp := <-responseChan
	return &batch{
		queue:    b,
		entries:  resp.entries,
		doneChan: resp.ackChan,
	}, nil
}

func (b *broker) Metrics() (queue.Metrics, error) {
	responseChan := make(chan memQueueMetrics, 1)
	select {
	case <-b.done:
		return queue.Metrics{}, io.EOF
	case b.metricChan <- metricsRequest{
		responseChan: responseChan}:
	}

	// FixME
	_ = <-responseChan
	return queue.Metrics{}, nil
}

var ackChanPool = sync.Pool{
	New: func() interface{} {
		return &batchACKState{
			doneChan: make(chan batchDoneMsg, 1),
		}
	},
}

func newBatchACKState(start, count int, entries []queueEntry) *batchACKState {
	ch := ackChanPool.Get().(*batchACKState)
	ch.next = nil
	ch.start = start
	ch.count = count
	ch.entries = entries
	return ch
}

func releaseACKChan(c *batchACKState) {
	c.next = nil
	ackChanPool.Put(c)
}

type pushRequest struct {
	event interface{}

	// the producer that generated this event, or nil if this producer does not require ack callbacks.
	producer *ackProducer

	// The index of the event in this producer only. Used to condense multitple acks for a producer to a single callback call
	producerID producerID
	resp       chan queue.EntryID
}

type producerCancelRequest struct {
	producer *ackProducer
	resp     chan producerCancelResponse
}

type producerCancelResponse struct {
	removed int
}

type getRequest struct {
	entryCount   int              // request entryCount events from the broker
	responseChan chan getResponse // channel to send response to
}

type getResponse struct {
	ackChan chan batchDoneMsg
	entries []queueEntry
}

type batchDoneMsg struct{}

type metricsRequest struct {
	responseChan chan memQueueMetrics
}

// memQueueMetrics tracks metrics that are returned by the individual memory queue impl
type memQueueMetrics struct {
	// the size of items in the queue
	currentQueueSize int
	// the number of items that have been read by a consumer but no yet acked
	occupiedRead  int
	oldestEntryID queue.EntryID
}

func AdjustInputQueueSize(requested, mainQueueSize int) (actual int) {
	actual = requested
	if max := int(float64(mainQueueSize) * maxInputQueueSizeRatio); mainQueueSize > 0 && actual > max {
		actual = max
	}

	if actual < minInputQueueSize {
		actual = minInputQueueSize
	}
	return actual
}
