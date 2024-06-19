package sync

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/net/streampool"
)

type RequestManager interface {
	QueueRequest(rq syncdeps.Request) error
	SendRequest(ctx context.Context, rq syncdeps.Request, collector syncdeps.ResponseCollector) error
	HandleStreamRequest(ctx context.Context, rq syncdeps.Request, stream drpc.Stream) error
	Close()
}

type StreamResponse struct {
	Stream     drpc.Stream
	Connection drpc.Conn
}

type requestManager struct {
	requestPool   RequestPool
	incomingGuard *guard
	handler       syncdeps.SyncHandler
}

func NewRequestManager(handler syncdeps.SyncHandler) RequestManager {
	return &requestManager{
		requestPool:   NewRequestPool(),
		handler:       handler,
		incomingGuard: newGuard(),
	}
}

func (r *requestManager) SendRequest(ctx context.Context, rq syncdeps.Request, collector syncdeps.ResponseCollector) error {
	return r.handler.SendStreamRequest(ctx, rq, func(stream drpc.Stream) error {
		calledOnce := false
		for {
			resp := collector.NewResponse()
			err := stream.MsgRecv(resp, streampool.EncodingProto)
			if err != nil {
				if errors.Is(err, io.EOF) && calledOnce {
					return nil
				}
				return err
			}
			err = collector.CollectResponse(ctx, rq.PeerId(), rq.ObjectId(), resp)
			if err != nil {
				return err
			}
			calledOnce = true
		}
	})
}

func (r *requestManager) QueueRequest(rq syncdeps.Request) error {
	return r.requestPool.QueueRequestAction(rq.PeerId(), rq.ObjectId(), func(ctx context.Context) {
		err := r.handler.ApplyRequest(ctx, rq, r)
		if err != nil {
			log.Error("failed to apply request", zap.Error(err))
		}
	})
}

func (r *requestManager) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, stream drpc.Stream) error {
	if !r.incomingGuard.TryTake(fullId(rq.PeerId(), rq.ObjectId())) {
		return nil
	}
	defer r.incomingGuard.Release(fullId(rq.PeerId(), rq.ObjectId()))
	newRq, err := r.handler.HandleStreamRequest(ctx, rq, func(resp proto.Message) error {
		return stream.MsgSend(resp, streampool.EncodingProto)
	})
	// here is a little bit non-standard decision, because we can return error but still can queue the request
	if newRq != nil {
		rqErr := r.QueueRequest(newRq)
		if rqErr != nil {
			log.Debug("failed to queue request", zap.Error(err))
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (r *requestManager) Close() {
	r.requestPool.Close()
}

func fullId(peerId, objectId string) string {
	return strings.Join([]string{peerId, objectId}, "-")
}
