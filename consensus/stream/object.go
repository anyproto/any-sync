package stream

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"sync"
)

// object is a cache entry that holds the actual log state and maintains added streams
type object struct {
	logId   []byte
	records []consensus.Record

	streams map[uint64]*Stream

	mu sync.Mutex
}

// AddRecords adds new records to the log and sends new records to streams
// The records source is db and called via stream.Service
func (o *object) AddRecords(recs []consensus.Record) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(recs) <= len(o.records) {
		return
	}
	diff := recs[0 : len(recs)-len(o.records)]
	o.records = recs
	for _, st := range o.streams {
		_ = st.AddRecords(o.logId, diff)
	}
}

// Records returns all log records
func (o *object) Records() []consensus.Record {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.records
}

// AddStream adds stream to the object
func (o *object) AddStream(s *Stream) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.streams[s.id] = s
	_ = s.AddRecords(o.logId, o.records)
	return
}

// RemoveStream remove stream from object
func (o *object) RemoveStream(id uint64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.streams, id)
}

// Locked indicates that object can be garbage collected
func (o *object) Locked() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.streams) > 0
}

func (o *object) Close() (err error) {
	return nil
}
