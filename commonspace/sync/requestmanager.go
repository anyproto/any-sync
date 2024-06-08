package sync

import (
	"context"
	"fmt"
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
	requestPool RequestPool
	handler     syncdeps.SyncHandler
}

func NewRequestManager(handler syncdeps.SyncHandler) RequestManager {
	return &requestManager{
		requestPool: NewRequestPool(),
		handler:     handler,
	}
}

func (r *requestManager) QueueRequest(rq syncdeps.Request) error {
	return r.requestPool.QueueRequestAction(rq.PeerId(), rq.ObjectId(), func(ctx context.Context) {
		fmt.Println("starting stream request", rq.PeerId(), rq.ObjectId())
		defer fmt.Println("ending stream request", rq.PeerId(), rq.ObjectId())
		err := r.handler.SendStreamRequest(ctx, rq, func(stream drpc.Stream) error {
			for {
				resp := r.handler.NewResponse()
				fmt.Println("receiving message", rq.PeerId(), rq.ObjectId())
				err := stream.MsgRecv(resp, streampool.EncodingProto)
				fmt.Println("received message", rq.PeerId(), rq.ObjectId(), err)
				if err != nil {
					return err
				}
				fmt.Println("handling response", rq.PeerId(), rq.ObjectId())
				err = r.handler.HandleResponse(ctx, rq.PeerId(), rq.ObjectId(), resp)
				fmt.Println("handled response", rq.PeerId(), rq.ObjectId(), err)
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
	fmt.Println("handling stream request", rq.PeerId(), rq.ObjectId())
	defer r.requestPool.Release(rq.PeerId(), rq.ObjectId())
	newRq, err := r.handler.HandleStreamRequest(ctx, rq, func(resp proto.Message) error {
		fmt.Println("sending response", rq.PeerId(), rq.ObjectId())
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
