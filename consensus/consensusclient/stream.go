package consensusclient

import (
	"context"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/cheggaaa/mb/v3"
	"sync"
)

func runStream(rpcStream consensusproto.DRPCConsensus_LogWatchClient) *stream {
	st := &stream{
		rpcStream: rpcStream,
		mb:        mb.New[*consensusproto.LogWatchEvent](100),
	}
	go st.readStream()
	return st
}

type stream struct {
	rpcStream consensusproto.DRPCConsensus_LogWatchClient
	mb        *mb.MB[*consensusproto.LogWatchEvent]
	mu        sync.Mutex
	err       error
}

func (s *stream) WatchIds(logIds [][]byte) (err error) {
	return s.rpcStream.Send(&consensusproto.LogWatchRequest{
		WatchIds: logIds,
	})
}

func (s *stream) UnwatchIds(logIds [][]byte) (err error) {
	return s.rpcStream.Send(&consensusproto.LogWatchRequest{
		UnwatchIds: logIds,
	})
}

func (s *stream) WaitLogs() []*consensusproto.LogWatchEvent {
	events, _ := s.mb.Wait(context.TODO())
	return events
}

func (s *stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *stream) readStream() {
	defer s.Close()
	for {
		event, err := s.rpcStream.Recv()
		if err != nil {
			s.mu.Lock()
			s.err = err
			s.mu.Unlock()
			return
		}
		if err = s.mb.Add(s.rpcStream.Context(), event); err != nil {
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
