package queue

type EntryID int64

type Queue interface {
	Close() error
	BufferConfig() BufferConfig
	Producer(cfg ProducerConfig) Producer
	Get(eventCount int) (Batch, error)
	Metrics() (Metrics, error)
}

type Producer interface {
	Publish(event interface{}) (EntryID, bool)
	TryPublish(event interface{}) (EntryID, bool)
	Cancel() int
}

type Batch interface {
	Count() int
	Entry(i int) interface{}
	FreeEntries()
	Done()
}

// Metrics is a set of basic-user friendly metrics that
// report the current state of the queue. These metrics are meant to be
// relatively generic and high-level, and when reported directly, can be
// comprehensible to a user.
type Metrics struct {
	OldestEntryID EntryID
}

// BufferConfig returns the pipelines buffering settings,
// for the pipeline to use.
// In case of the pipeline itself storing events for reporting ACKs to clients,
// but still dropping events, the pipeline can use the buffer information,
// to define an upper bound of events being active in the pipeline.
type BufferConfig struct {
	// MaxEvents is the maximum number of events the queue can hold at capacity.
	// A value <= 0 means there is no fixed limit.
	MaxEvents int
}

// ProducerConfig as used by the Pipeline to configure some custom callbacks
// between pipeline and queue.
type ProducerConfig struct {
	// if ACK is set, the callback will be called with number of events produced
	// by the producer instance and being ACKed by the queue.
	ACK func(count int)

	// OnDrop provided to the queue, to report events being silently dropped by
	// the queue. For example an async producer close and publish event,
	// with close happening early might result in the event being dropped. The callback
	// gives a queue user a chance to keep track of total number of events
	// being buffered by the queue.
	OnDrop func(interface{})

	// DropOnCancel is a hint to the queue to drop events if the producer disconnects
	// via Cancel.
	DropOnCancel bool
}

type ACKListener interface {
	OnACK(eventCount int) // number of consecutively published events acked by producers
}
