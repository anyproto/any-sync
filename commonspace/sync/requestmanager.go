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
	//heads   []string
	//changes []*treechangeproto.RawTreeChangeWithId
	//root    *treechangeproto.RawTreeChangeWithId
}

type Response interface {
	//heads   []string
	//changes []*treechangeproto.RawTreeChangeWithId
	//root    *treechangeproto.RawTreeChangeWithId
}

type RequestManager interface {
	QueueRequest(peerId, objectId string, rq Request) error
	HandleRequest(peerId, objectId string, rq Request) error
	HandleStreamRequest(peerId, objectId string, rq Request, stream drpc.Stream) error
}

type RequestHandler interface {
	HandleRequest(peerId, objectId string, rq Request) (Request, error)
	HandleStreamRequest(peerId, objectId string, rq Request, send func(resp proto.Message) error) (Request, error)
}

type StreamResponse struct {
	Stream     drpc.Stream
	Connection drpc.Conn
}

type RequestSender interface {
	SendRequest(peerId, objectId string, rq Request) (resp Response, err error)
	SendStreamRequest(peerId, objectId string, rq Request, receive func(stream drpc.Stream) error) (err error)
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

func (r *requestManager) QueueRequest(peerId, objectId string, rq Request) error {
	return r.requestPool.QueueRequestAction(peerId, objectId, func() {
		err := r.requestSender.SendStreamRequest(peerId, objectId, rq, func(stream drpc.Stream) error {
			for {
				resp := r.responseHandler.NewResponse()
				err := stream.MsgRecv(resp, streampool.EncodingProto)
				if err != nil {
					return err
				}
				err = r.responseHandler.HandleResponse(peerId, objectId, resp)
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

func (r *requestManager) HandleRequest(peerId, objectId string, rq Request) error {
	id := fullId(peerId, objectId)
	r.mx.Lock()
	if _, ok := r.currentRequests[id]; ok {
		r.mx.Unlock()
		return nil
	}
	r.currentRequests[id] = struct{}{}
	r.mx.Unlock()
	defer func() {
		r.mx.Lock()
		delete(r.currentRequests, id)
		r.mx.Unlock()
	}()
	newRq, err := r.requestHandler.HandleRequest(peerId, objectId, rq)
	if err != nil {
		return err
	}
	if newRq != nil {
		return r.QueueRequest(peerId, objectId, newRq)
	}
	return nil
}

func (r *requestManager) HandleStreamRequest(peerId, objectId string, rq Request, stream drpc.Stream) error {
	id := fullId(peerId, objectId)
	r.mx.Lock()
	if _, ok := r.currentRequests[id]; ok {
		r.mx.Unlock()
		return nil
	}
	r.currentRequests[id] = struct{}{}
	r.mx.Unlock()
	defer func() {
		r.mx.Lock()
		delete(r.currentRequests, id)
		r.mx.Unlock()
	}()
	newRq, err := r.requestHandler.HandleStreamRequest(peerId, objectId, rq, func(resp proto.Message) error {
		return stream.MsgSend(resp, streampool.EncodingProto)
	})
	if err != nil {
		return err
	}
	if newRq != nil {
		return r.QueueRequest(peerId, objectId, newRq)
	}
	return nil
}

func fullId(peerId, objectId string) string {
	return strings.Join([]string{peerId, objectId}, "-")
}
