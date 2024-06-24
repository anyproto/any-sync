package synctest

import (
	"context"
	"fmt"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/streampool"
)

const RpcName = "rpcserver"

type SyncService interface {
	app.Component
	HandleMessage(ctx context.Context, peerId string, msg drpc.Message) error
	HandleStreamRequest(ctx context.Context, req syncdeps.Request, stream drpc.Stream) error
	QueueRequest(ctx context.Context, rq syncdeps.Request) error
}

type RpcServer struct {
	streamPool  streampool.StreamPool
	syncService SyncService
	ownId       string
}

func NewRpcServer() *RpcServer {
	return &RpcServer{}
}

func (r *RpcServer) CounterStreamRequest(request *synctestproto.CounterRequest, stream synctestproto.DRPCCounterSync_CounterStreamRequestStream) error {
	peerId, err := peer.CtxPeerId(stream.Context())
	if err != nil {
		return err
	}
	fmt.Println("received a request", peerId, request.ObjectId)
	req := NewCounterRequest(peerId, request.ObjectId, request.ExistingValues)
	return r.syncService.HandleStreamRequest(stream.Context(), req, stream)
}

func (r *RpcServer) CounterStream(stream synctestproto.DRPCCounterSync_CounterStreamStream) error {
	fmt.Println("received a stream")
	return r.streamPool.ReadStream(stream)
}

func (r *RpcServer) Init(a *app.App) (err error) {
	serv := a.MustComponent(server.CName).(*rpctest.TestServer)
	r.streamPool = a.MustComponent(streampool.CName).(streampool.StreamPool)
	r.syncService = a.MustComponent("common.commonspace.sync").(SyncService)
	r.ownId = a.MustComponent(PeerName).(*PeerProvider).myPeer
	return synctestproto.DRPCRegisterCounterSync(serv, r)
}

func (r *RpcServer) Name() (name string) {
	return RpcName
}
