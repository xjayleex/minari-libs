package memqueue

type batchBuffer struct {
	next    *batchBuffer
	flushed bool
	entries []queueEntry
}

func newBatchBuffer(size int) *batchBuffer {
	b := &batchBuffer{}
	b.entries = make([]queueEntry, 0, size)
	return b
}

func (b *batchBuffer) add(entry queueEntry) {
	b.entries = append(b.entries, entry)
}

func (b *batchBuffer) length() int {
	return len(b.entries)
}

func (b *batchBuffer) cancel(producer *ackProducer) int {
	entries := b.entries[:0]

	removedCount := 0
	for _, entry := range b.entries {
		if entry.producer == producer {
			removedCount++
			continue
		}
		entries = append(entries, entry)
	}
	b.entries = entries
	return removedCount
}
