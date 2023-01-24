package streampool

import (
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
	if err = sr.stream.MsgSend(msg, EncodingProto); err != nil {
		sr.streamClose()
	}
	return err
}

func (sr *stream) readLoop() error {
	defer func() {
		sr.streamClose()
	}()
	sr.l.Debug("stream read started")
	for {
		msg := sr.pool.handler.NewReadMessage()
		if err := sr.stream.MsgRecv(msg, EncodingProto); err != nil {
			sr.l.Info("msg receive error", zap.Error(err))
			return err
		}
		if err := sr.pool.HandleMessage(sr.stream.Context(), sr.peerId, msg); err != nil {
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
