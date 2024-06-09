package sync

import (
	"context"
	"strings"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/net/streampool"
)

type RequestManager interface {
	QueueRequest(rq syncdeps.Request) error
	SendRequest(ctx context.Context, rq syncdeps.Request, collector ResponseCollector) error
	HandleStreamRequest(ctx context.Context, rq syncdeps.Request, stream drpc.Stream) error
}

type ResponseCollector interface {
	CollectResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error
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

func (r *requestManager) SendRequest(ctx context.Context, rq syncdeps.Request, collector ResponseCollector) error {
	return r.handler.SendStreamRequest(ctx, rq, func(stream drpc.Stream) error {
		for {
			resp := r.handler.NewResponse()
			err := stream.MsgRecv(resp, streampool.EncodingProto)
			if err != nil {
				return err
			}
			err = collector.CollectResponse(ctx, rq.PeerId(), rq.ObjectId(), resp)
			if err != nil {
				return err
			}
		}
	})
}

func (r *requestManager) QueueRequest(rq syncdeps.Request) error {
	return r.requestPool.QueueRequestAction(rq.PeerId(), rq.ObjectId(), func(ctx context.Context) {
		err := r.handler.SendStreamRequest(ctx, rq, func(stream drpc.Stream) error {
			for {
				resp := r.handler.NewResponse()
				err := stream.MsgRecv(resp, streampool.EncodingProto)
				if err != nil {
					return err
				}
				err = r.handler.HandleResponse(ctx, rq.PeerId(), rq.ObjectId(), resp)
				if err != nil {
					return err
				}
			}
		})
		if err != nil {
			log.Warn("failed to receive request", zap.Error(err))
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
	if err != nil {
		return err
	}
	if newRq != nil {
		return r.QueueRequest(newRq)
	}
	return nil
}

func fullId(peerId, objectId string) string {
	return strings.Join([]string{peerId, objectId}, "-")
}
