package memqueue

import (
	"github.com/xjayleex/minari-libs/logpack"
	"github.com/xjayleex/minari-libs/queue"
)

type producerID uint64

type produceState struct {
	cb       ackHandler
	dropCB   func(interface{})
	canceled bool
	lastACK  producerID
}

type ackHandler func(count int)

func newProducer(b *broker, callback ackHandler, dropCallback func(interface{}), dropOnCancel bool) queue.Producer {
	openState := openState{
		log:    b.logger,
		done:   make(chan struct{}),
		events: b.pushChan,
	}

	if callback != nil {
		return &ackProducer{
			broker:        b,
			dropOnCancel:  dropOnCancel,
			producedCount: 0,
			state: produceState{
				cb:     callback,
				dropCB: dropCallback,
			},
			openState: openState,
		}
	}

	return &forgetfulProducer{
		broker:    b,
		openState: openState,
	}
}

type ackProducer struct {
	broker        *broker
	dropOnCancel  bool
	producedCount uint64
	state         produceState
	openState     openState
}

func (p *ackProducer) makePushRequest(event interface{}) pushRequest {
	resp := make(chan queue.EntryID, 1)
	return pushRequest{
		event:    event,
		producer: p,
		// Note that default lastACK of 0 is a valid initial state and 1 is the first real id.
		producerID: producerID(p.producedCount + 1),
		resp:       resp,
	}
}

func (p *ackProducer) Publish(event interface{}) (queue.EntryID, bool) {
	id, published := p.openState.publish(p.makePushRequest(event))
	if published {
		p.producedCount += 1
	}
	return id, published
}

func (p *ackProducer) TryPublish(event interface{}) (queue.EntryID, bool) {
	id, published := p.openState.tryPublish(p.makePushRequest(event))
	if published {
		p.producedCount += 1
	}
	return id, published
}

func (p *ackProducer) Cancel() int {
	p.openState.Close()

	if p.dropOnCancel {
		respCh := make(chan producerCancelResponse)
		p.broker.cancelChan <- producerCancelRequest{
			producer: p,
			resp:     respCh,
		}

		resp := <-respCh
		return resp.removed
	}

	return 0
}

type forgetfulProducer struct {
	broker    *broker
	openState openState
}

func (p *forgetfulProducer) makePushRequest(event interface{}) pushRequest {
	resp := make(chan queue.EntryID, 1)
	return pushRequest{
		event: event,
		resp:  resp,
	}
}

// Publish implements queue.Producer
func (p *forgetfulProducer) Publish(event interface{}) (queue.EntryID, bool) {
	return p.openState.publish(p.makePushRequest(event))
}

// TryPublish implements queue.Producer
func (p *forgetfulProducer) TryPublish(event interface{}) (queue.EntryID, bool) {
	return p.openState.tryPublish(p.makePushRequest(event))
}

// Cancel implements queue.Producer
func (p *forgetfulProducer) Cancel() int {
	p.openState.Close()
	return 0
}

// openstate implementation
type openState struct {
	log    logpack.Logger
	done   chan struct{}
	events chan pushRequest
}

func (st *openState) Close() {
	close(st.done)
}

func (st *openState) publish(req pushRequest) (queue.EntryID, bool) {
	select {
	case st.events <- req:
		return <-req.resp, true
	case <-st.done:
		st.events = nil
		return 0, false
	}
}

func (st *openState) tryPublish(req pushRequest) (queue.EntryID, bool) {
	select {
	case st.events <- req:
		return <-req.resp, true
	case <-st.done:
		st.events = nil
		return 0, false
	default:
		st.log.Debugf("Dropping event, queue is blocked")
		return 0, false
	}
}
