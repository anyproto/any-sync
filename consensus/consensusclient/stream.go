package consensusclient

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"github.com/cheggaaa/mb/v2"
)

func runStream(rpcStream consensusproto.DRPCConsensus_WatchLogClient) *stream {
	st := &stream{
		rpcStream: rpcStream,
		mb:        mb.New((*consensusproto.WatchLogEvent)(nil), 100),
	}
	go st.readStream()
	return st
}

type stream struct {
	rpcStream consensusproto.DRPCConsensus_WatchLogClient
	mb        *mb.MB[*consensusproto.WatchLogEvent]
	err       error
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

func (s *stream) WaitLogs() []*consensusproto.WatchLogEvent {
	return s.mb.Wait()
}

func (s *stream) Err() error {
	return s.err
}

func (s *stream) readStream() {
	defer s.Close()
	for {
		event, err := s.rpcStream.Recv()
		if err != nil {
			s.err = err
			return
		}
		if err = s.mb.Add(event); err != nil {
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
