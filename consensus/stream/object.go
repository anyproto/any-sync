package stream

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/cheggaaa/mb/v2"
	"sync"
)

type object struct {
	logId   []byte
	records []consensus.Record

	streams map[uint32]*stream

	lastStreamId uint32

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
		st.AddRecords(diff)
	}
}

func (o *object) Records() []consensus.Record {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.records
}

func (o *object) NewStream() Stream {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.lastStreamId++
	st := &stream{
		id:      o.lastStreamId,
		obj:     o,
		records: o.records,
		mb:      mb.New(consensus.Record{}, 100),
	}
	o.streams[st.id] = st
	return st
}

func (o *object) Locked() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.streams) > 0
}

func (o *object) removeStream(id uint32) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.streams, id)
}

func (o *object) Close() (err error) {
	return nil
}
