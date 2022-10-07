package stream

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/cheggaaa/mb/v2"
)

type stream struct {
	id      uint32
	obj     *object
	records []consensus.Record
	mb      *mb.MB[consensus.Record]
}

func (s *stream) LogId() []byte {
	return s.obj.logId
}

func (s *stream) AddRecords(records []consensus.Record) {
	_ = s.mb.Add(records...)
}

func (s *stream) Records() []consensus.Record {
	return s.records
}

func (s *stream) WaitRecords() []consensus.Record {
	return s.mb.Wait()
}

func (s *stream) Close() {
	_ = s.mb.Close()
	s.obj.removeStream(s.id)
}
