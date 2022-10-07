package consensusclient

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"github.com/cheggaaa/mb/v2"
)

type Stream interface {
	WatchIds(logIds [][]byte) (err error)
	UnwatchIds(logIds [][]byte) (err error)
	WaitLogs() []*consensusproto.Log
	Close() error
}

func runStream(rpcStream consensusproto.DRPCConsensus_WatchLogClient) Stream {
	st := &stream{
		rpcStream: rpcStream,
		mb:        mb.New((*consensusproto.Log)(nil), 100),
	}
	go st.readStream()
	return st
}

type stream struct {
	rpcStream consensusproto.DRPCConsensus_WatchLogClient
	mb        *mb.MB[*consensusproto.Log]
}

func (s *stream) WatchIds(logIds [][]byte) (err error) {
	return s.rpcStream.Send(&consensusproto.WatchLogRequest{
		WatchIds: logIds,
	})
}

func (s *stream) UnwatchIds(logIds [][]byte) (err error) {
	return s.rpcStream.Send(&consensusproto.WatchLogRequest{
		UnwatchIds: logIds,
	})
}

func (s *stream) WaitLogs() []*consensusproto.Log {
	return s.mb.Wait()
}

func (s *stream) readStream() {
	defer s.Close()
	for {
		event, err := s.rpcStream.Recv()
		if err != nil {
			return
		}
		if err = s.mb.Add(&consensusproto.Log{
			Id:      event.LogId,
			Records: event.Records,
		}); err != nil {
			return
		}
	}
}

func (s *stream) Close() error {
	if err := s.mb.Close(); err == nil {
		return s.rpcStream.Close()
	}
	return nil
}
