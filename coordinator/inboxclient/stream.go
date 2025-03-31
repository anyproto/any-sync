package inboxclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
)

func runStream(rpcStream coordinatorproto.DRPCCoordinator_InboxNotifySubscribeClient) *stream {
	st := &stream{
		rpcStream: rpcStream,
		mb:        mb.New[*coordinatorproto.InboxNotifySubscribeEvent](1),
	}
	go st.readStream()
	return st
}

var ErrShutdown = errors.New("stream shutted down")

type stream struct {
	rpcStream  coordinatorproto.DRPCCoordinator_InboxNotifySubscribeClient
	mb         *mb.MB[*coordinatorproto.InboxNotifySubscribeEvent]
	mu         sync.Mutex
	isShutdown atomic.Bool
}

// if close, reconnect
// if shutdown, don't try more
func (s *stream) WaitNotifyEvents() (*coordinatorproto.InboxNotifySubscribeEvent, error) {
	event, err := s.mb.WaitOne(context.TODO())
	if err != nil {
		if s.isShutdown.Load() {
			return nil, ErrShutdown
		}
		return nil, err
	}

	return event, nil
}

func (s *stream) readStream() {
	defer s.Close()
	for {
		event, err := s.rpcStream.Recv()
		log.Info("read stream", zap.String("event", fmt.Sprintf("%#v", event)))
		if err != nil {
			log.Error("read stream err", zap.Error(err))
			s.close()
			return
		}
		log.Info("read stream, mb add")
		if err = s.mb.TryAdd(event); err != nil {
			if err == mb.ErrOverflowed {
				continue
			}
			return
		}
	}
}

func (s *stream) close() {
	_ = s.mb.Close()
	_ = s.rpcStream.Close()
}
func (s *stream) Close() error {
	if s.isShutdown.CompareAndSwap(false, true) {
		s.close()
	}
	return nil
}
