package pubsub

import (
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
)

// rpcHandler adapts Service to the generated DRPC server interface.
type rpcHandler struct {
	s Service
}

func (r rpcHandler) PubSubStream(stream pubsubproto.DRPCPubSub_PubSubStreamStream) error {
	return r.s.HandleStream(stream)
}

// RegisterRpc registers the pubsub DRPC service on the given mux.
func RegisterRpc(mux drpc.Mux, s Service) error {
	return pubsubproto.DRPCRegisterPubSub(mux, rpcHandler{s: s})
}
