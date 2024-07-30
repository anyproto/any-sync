package sync

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool"
)

type RequestManager interface {
	QueueRequest(rq syncdeps.Request) error
	SendRequest(ctx context.Context, rq syncdeps.Request, collector syncdeps.ResponseCollector) error
	HandleStreamRequest(ctx context.Context, rq syncdeps.Request, stream drpc.Stream) error
	HandleDeprecatedObjectSync(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error)
	Close()
}

type StreamResponse struct {
	Stream     drpc.Stream
	Connection drpc.Conn
}

type requestManager struct {
	requestPool   RequestPool
	incomingGuard *guard
	limit         *Limit
	handler       syncdeps.SyncHandler
	metric        syncdeps.QueueSizeUpdater
}

func NewRequestManager(handler syncdeps.SyncHandler, metric syncdeps.QueueSizeUpdater) RequestManager {
	return &requestManager{
		requestPool:   NewRequestPool(),
		limit:         NewLimit(10),
		handler:       handler,
		incomingGuard: newGuard(0),
		metric:        metric,
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
			size := resp.MsgSize()
			r.metric.UpdateQueueSize(size, syncdeps.MsgTypeReceivedResponse, true)
			err = collector.CollectResponse(ctx, rq.PeerId(), rq.ObjectId(), resp)
			r.metric.UpdateQueueSize(size, syncdeps.MsgTypeReceivedResponse, false)
			if err != nil {
				return err
			}
			calledOnce = true
		}
	})
}

func (r *requestManager) QueueRequest(rq syncdeps.Request) error {
	size := rq.MsgSize()
	r.metric.UpdateQueueSize(size, syncdeps.MsgTypeOutgoingRequest, true)
	return r.requestPool.QueueRequestAction(rq.PeerId(), rq.ObjectId(), func(ctx context.Context) {
		err := r.handler.ApplyRequest(ctx, rq, r)
		if err != nil {
			log.Error("failed to apply request", zap.Error(err))
		}
	}, func() {
		r.metric.UpdateQueueSize(size, syncdeps.MsgTypeOutgoingRequest, false)
	})
}

func (r *requestManager) HandleDeprecatedObjectSync(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error) {
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	if !r.incomingGuard.TryTake(fullId(peerId, req.ObjectId)) {
		return nil, spacesyncproto.ErrUnexpected
	}
	defer r.incomingGuard.Release(fullId(peerId, req.ObjectId))
	return r.handler.HandleDeprecatedObjectSync(ctx, req)
}

func (r *requestManager) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, stream drpc.Stream) error {
	size := rq.MsgSize()
	if !r.incomingGuard.TryTake(fullId(rq.PeerId(), rq.ObjectId())) {
		return spacesyncproto.ErrDuplicateRequest
	}
	defer r.incomingGuard.Release(fullId(rq.PeerId(), rq.ObjectId()))
	if !r.limit.Take(rq.PeerId()) {
		return spacesyncproto.ErrTooManyRequestsFromPeer
	}
	defer r.limit.Release(rq.PeerId())
	r.metric.UpdateQueueSize(size, syncdeps.MsgTypeIncomingRequest, true)
	defer r.metric.UpdateQueueSize(size, syncdeps.MsgTypeIncomingRequest, false)
	newRq, err := r.handler.HandleStreamRequest(ctx, rq, r.metric, func(resp proto.Message) error {
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
