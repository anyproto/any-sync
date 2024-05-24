package sync

import (
	"context"
	"strings"
	"sync"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/net/streampool"
)

type Request interface {
	PeerId() string
	ObjectId() string
}

type Response interface {
	// heads   []string
	// changes []*treechangeproto.RawTreeChangeWithId
	// root    *treechangeproto.RawTreeChangeWithId
}

type RequestManager interface {
	QueueRequest(rq Request) error
	HandleStreamRequest(rq Request, stream drpc.Stream) error
}

type RequestHandler interface {
	HandleRequest(rq Request) (Request, error)
	HandleStreamRequest(rq Request, send func(resp proto.Message) error) (Request, error)
}

type StreamResponse struct {
	Stream     drpc.Stream
	Connection drpc.Conn
}

type RequestSender interface {
	SendRequest(rq Request) (resp Response, err error)
	SendStreamRequest(rq Request, receive func(stream drpc.Stream) error) (err error)
}

type ResponseHandler interface {
	NewResponse() Response
	HandleResponse(peerId, objectId string, resp Response) error
}

type requestManager struct {
	requestPool     RequestPool
	requestHandler  RequestHandler
	responseHandler ResponseHandler
	requestSender   RequestSender
	currentRequests map[string]struct{}
	mx              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	wait            chan struct{}
}

func (r *requestManager) QueueRequest(rq Request) error {
	return r.requestPool.QueueRequestAction(rq.PeerId(), rq.ObjectId(), func() {
		err := r.requestSender.SendStreamRequest(rq, func(stream drpc.Stream) error {
			for {
				resp := r.responseHandler.NewResponse()
				err := stream.MsgRecv(resp, streampool.EncodingProto)
				if err != nil {
					return err
				}
				err = r.responseHandler.HandleResponse(rq.PeerId(), rq.ObjectId(), resp)
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

func (r *requestManager) HandleStreamRequest(rq Request, stream drpc.Stream) error {
	if !r.requestPool.TryTake(rq.PeerId(), rq.ObjectId()) {
		return nil
	}
	defer r.requestPool.Release(rq.PeerId(), rq.ObjectId())
	newRq, err := r.requestHandler.HandleStreamRequest(rq, func(resp proto.Message) error {
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
