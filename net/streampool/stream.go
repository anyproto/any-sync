package streampool

import (
	"fmt"
	"go.uber.org/zap"
	"storj.io/drpc"
	"sync/atomic"
)

type stream struct {
	peerId   string
	stream   drpc.Stream
	pool     *streamPool
	streamId uint32
	closed   atomic.Bool
	l        *zap.Logger
	tags     []string
}

func (sr *stream) write(msg drpc.Message) (err error) {
	defer func() {
		sr.l.Debug("write", zap.String("msg", msg.(fmt.Stringer).String()), zap.Error(err))
	}()
	if err = sr.stream.MsgSend(msg, EncodingProto); err != nil {
		sr.l.Info("stream write error", zap.Error(err))
		sr.streamClose()
	}
	return err
}

func (sr *stream) readLoop() error {
	defer func() {
		sr.streamClose()
	}()
	for {
		msg := sr.pool.handler.NewReadMessage()
		if err := sr.stream.MsgRecv(msg, EncodingProto); err != nil {
			sr.l.Info("msg receive error", zap.Error(err))
			return err
		}
		sr.l.Debug("read msg", zap.String("msg", msg.(fmt.Stringer).String()))
		if err := sr.pool.handler.HandleMessage(sr.stream.Context(), sr.peerId, msg); err != nil {
			sr.l.Info("msg handle error", zap.Error(err))
			return err
		}
	}
}

func (sr *stream) streamClose() {
	if !sr.closed.Swap(true) {
		_ = sr.stream.Close()
		sr.pool.removeStream(sr.streamId)
	}
}
