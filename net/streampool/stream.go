package streampool

import (
	"context"
	"github.com/anytypeio/any-sync/app/logger"
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
	l        logger.CtxLogger
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
		ctx := streamCtx(context.Background(), sr.streamId, sr.peerId)
		ctx = logger.CtxWithFields(ctx, zap.String("rootOp", "streamMessage"), zap.String("peerId", sr.peerId))
		if err := sr.pool.HandleMessage(ctx, sr.peerId, msg); err != nil {
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
