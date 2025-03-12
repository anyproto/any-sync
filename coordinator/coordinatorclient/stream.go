package coordinatorclient

import (
	"context"
	"sync"

	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/cheggaaa/mb/v3"
)

func runStream(rpcStream coordinatorproto.DRPCCoordinator_InboxNotifySubscribeClient) *stream {
	st := &stream{
		rpcStream: rpcStream,
		mb:        mb.New[*coordinatorproto.InboxNotifySubscribeEvent](100),
	}
	go st.readStream()
	return st
}

type stream struct {
	rpcStream coordinatorproto.DRPCCoordinator_InboxNotifySubscribeClient
	mb        *mb.MB[*coordinatorproto.InboxNotifySubscribeEvent]
	mu        sync.Mutex
	err       error
}

func (s *stream) WaitNotifyEvents() *coordinatorproto.InboxNotifySubscribeEvent {
	event, _ := s.mb.WaitOne(context.TODO())
	return event
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
