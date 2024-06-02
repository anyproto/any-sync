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
	HandleStreamRequest(ctx context.Context, rq syncdeps.Request, stream drpc.Stream) error
}

type StreamResponse struct {
	Stream     drpc.Stream
	Connection drpc.Conn
}

type requestManager struct {
	requestPool     RequestPool
	requestHandler  syncdeps.RequestHandler
	responseHandler syncdeps.ResponseHandler
	requestSender   syncdeps.RequestSender
}

func NewRequestManager(deps syncdeps.SyncDeps) RequestManager {
	return &requestManager{
		requestPool:     NewRequestPool(),
		requestHandler:  deps.RequestHandler,
		responseHandler: deps.ResponseHandler,
		requestSender:   deps.RequestSender,
	}
}

func (r *requestManager) QueueRequest(rq syncdeps.Request) error {
	return r.requestPool.QueueRequestAction(rq.PeerId(), rq.ObjectId(), func(ctx context.Context) {
		err := r.requestSender.SendStreamRequest(ctx, rq, func(stream drpc.Stream) error {
			for {
				resp := r.responseHandler.NewResponse()
				err := stream.MsgRecv(resp, streampool.EncodingProto)
				if err != nil {
					return err
				}
				err = r.responseHandler.HandleResponse(ctx, rq.PeerId(), rq.ObjectId(), resp)
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
	if !r.requestPool.TryTake(rq.PeerId(), rq.ObjectId()) {
		return nil
	}
	defer r.requestPool.Release(rq.PeerId(), rq.ObjectId())
	newRq, err := r.requestHandler.HandleStreamRequest(ctx, rq, func(resp proto.Message) error {
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
