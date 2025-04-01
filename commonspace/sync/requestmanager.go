package sync

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/anyproto/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/syncqueues"
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
	requestPool   syncqueues.ActionPool
	incomingGuard *syncqueues.Guard
	limit         *syncqueues.Limit
	handler       syncdeps.SyncHandler
	metric        syncdeps.QueueSizeUpdater
}

func NewRequestManager(handler syncdeps.SyncHandler, metric syncdeps.QueueSizeUpdater, requestPool syncqueues.ActionPool, limit *syncqueues.Limit) RequestManager {
	return &requestManager{
		requestPool:   requestPool,
		limit:         limit,
		handler:       handler,
		incomingGuard: syncqueues.NewGuard(),
		metric:        metric,
	}
}

func (r *requestManager) SendRequest(ctx context.Context, rq syncdeps.Request, collector syncdeps.ResponseCollector) error {
	return r.handler.SendStreamRequest(ctx, rq, func(stream drpc.Stream) error {
		calledOnce := false
		for {
			resp := collector.NewResponse()
			err := stream.MsgRecv(resp, peer.SnappyEnc{streampool.EncodingProto})
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
	r.requestPool.Add(rq.PeerId(), rq.ObjectId(), func(ctx context.Context) {
		r.handler.ApplyRequest(ctx, rq, r)
	}, func() {
		r.metric.UpdateQueueSize(size, syncdeps.MsgTypeOutgoingRequest, false)
	})
	return nil
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
		r.QueueRequest(newRq)
	}
	if err != nil {
		return err
	}
	return nil
}

func (r *requestManager) Close() {
}

func fullId(peerId, objectId string) string {
	return strings.Join([]string{peerId, objectId}, "-")
}
