package stream

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"sync"
)

type object struct {
	logId   []byte
	records []consensus.Record

	streams map[uint64]*Stream

	mu sync.Mutex
}

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

func (o *object) Records() []consensus.Record {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.records
}

func (o *object) AddStream(s *Stream) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.streams[s.id] = s
	_ = s.AddRecords(o.logId, o.records)
	return
}

func (o *object) Locked() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.streams) > 0
}

func (o *object) RemoveStream(id uint64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.streams, id)
}

func (o *object) Close() (err error) {
	return nil
}
