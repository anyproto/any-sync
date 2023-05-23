package streampool

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/util/multiqueue"
	"go.uber.org/zap"
	"storj.io/drpc"
	"sync/atomic"
)

type stream struct {
	peerId   string
	peerCtx  context.Context
	stream   drpc.Stream
	pool     *streamPool
	streamId uint32
	closed   atomic.Bool
	l        logger.CtxLogger
	queue    multiqueue.MultiQueue[drpc.Message]
	tags     []string
}

func (sr *stream) write(msg drpc.Message) (err error) {
	var queueId string
	if qId, ok := msg.(MessageQueueId); ok {
		queueId = qId.MessageQueueId()
		msg = qId.DrpcMessage()
	}
	return sr.queue.Add(sr.stream.Context(), queueId, msg)
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

func (sr *stream) writeToStream(msg drpc.Message) {
	if err := sr.stream.MsgSend(msg, EncodingProto); err != nil {
		sr.l.Warn("msg send error", zap.Error(err))
		sr.streamClose()
		return
	}
	return
}

func (sr *stream) streamClose() {
	if !sr.closed.Swap(true) {
		_ = sr.queue.Close()
		_ = sr.stream.Close()
		sr.pool.removeStream(sr.streamId)
	}
}
