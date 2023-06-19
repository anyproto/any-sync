package streampool

import (
	"context"
	"sync/atomic"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
	"storj.io/drpc"
)

type stream struct {
	peerId   string
	peerCtx  context.Context
	stream   drpc.Stream
	pool     *streamPool
	streamId uint32
	closed   atomic.Bool
	l        logger.CtxLogger
	queue    *mb.MB[drpc.Message]
	tags     []string
}

func (sr *stream) write(msg drpc.Message) (err error) {
	return sr.queue.TryAdd(msg)
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
		ctx := streamCtx(sr.peerCtx, sr.streamId, sr.peerId)
		ctx = logger.CtxWithFields(ctx, zap.String("peerId", sr.peerId))
		if err := sr.pool.handler.HandleMessage(ctx, sr.peerId, msg); err != nil {
			sr.l.Info("msg handle error", zap.Error(err))
			return err
		}
	}
}

func (sr *stream) writeLoop() {
	for {
		msg, err := sr.queue.WaitOne(sr.peerCtx)
		if err != nil {
			if err != mb.ErrClosed {
				sr.streamClose()
			}
			return
		}
		if err := sr.stream.MsgSend(msg, EncodingProto); err != nil {
			sr.l.Warn("msg send error", zap.Error(err))
			sr.streamClose()
			return
		}
	}
}

func (sr *stream) streamClose() {
	if !sr.closed.Swap(true) {
		_ = sr.queue.Close()
		_ = sr.stream.Close()
		sr.pool.removeStream(sr.streamId)
	}
}
