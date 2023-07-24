package memqueue

// ackLoop implements the brokers asynchronous ACK worker.
// Multiple concurrent ACKs from consecutive published batches will be batched up by the
// worker, to reduce the number of signals to return to the producer and the
// broker event loop.
// Producer ACKs are run in the ackLoop go-routine.
type ackLoop struct {
	broker *broker

	// A list of ACK channels given to queue consumers,
	// used to maintain sequencing of event acknowledgements.
	ackChans chanList

	processACK func(chanList, int)
}

func (l *ackLoop) run() {
	for {
		nextBatchChan := l.ackChans.nextBatchChannel()
		select {
		case <-l.broker.done:
			//  The queue is shutting down
			return
		case chanList := <-l.broker.scheduledACKs:
			l.ackChans.concat(&chanList)
		case <-nextBatchChan:
			// The oldest outstanding batch has been acknowledged, advance our
			// position as much as we can.
			l.handleBatchSig()
		}
	}
}

// handleBatchSig collects and handles a batch ACK/Cancel signal. handleBatchSig
// is run by the ackLoop.
func (l *ackLoop) handleBatchSig() int {
	list := l.collectAcked()

	count := 0
	for current := list.front(); current != nil; current = current.next {
		count += current.count
	}

	if count > 0 {
		if listener := l.broker.ackListener; listener != nil {
			listener.OnACK(count)
		}
		// report acks to waiting clients
		l.processACK(list, count)
	}

	for !list.empty() {
		releaseACKChan(list.pop())
	}

	// return final ACK to EventLoop, in order to clean up internal buffer
	l.broker.logger.Debug("ackloop: return ack to broker loop:", count)

	l.broker.logger.Debug("ackloop:  done send ack")
	return count
}

func (l *ackLoop) collectAcked() chanList {
	list := chanList{}

	acks := l.ackChans.pop()
	list.append(acks)

	done := false
	for !l.ackChans.empty() && !done {
		acks := l.ackChans.front()
		select {
		case <-acks.doneChan:
			list.append(l.ackChans.pop())
		default:
			done = true
		}
	}
	return list
}
