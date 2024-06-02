package synctest

import (
	"context"
	"fmt"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/multiqueue"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

const RpcName = "rpcserver"

type SyncService interface {
	app.Component
	GetQueue(peerId string) *multiqueue.Queue[drpc.Message]
	HandleMessage(ctx context.Context, peerId string, msg drpc.Message) error
	HandleStreamRequest(ctx context.Context, req syncdeps.Request, stream drpc.Stream) error
	QueueRequest(ctx context.Context, rq syncdeps.Request) error
}

type RpcServer struct {
	streamPool  streampool.StreamPool
	syncService SyncService
}

func NewRpcServer() *RpcServer {
	return &RpcServer{}
}

func (r *RpcServer) CounterStreamRequest(request *synctestproto.CounterRequest, stream synctestproto.DRPCCounterSync_CounterStreamRequestStream) error {
	fmt.Println(peer.CtxPeerId(stream.Context()))
	return nil
}

func (r *RpcServer) CounterStream(stream synctestproto.DRPCCounterSync_CounterStreamStream) error {
	return nil
}

func (r *RpcServer) Init(a *app.App) (err error) {
	serv := a.MustComponent(server.CName).(*rpctest.TestServer)
	r.streamPool = a.MustComponent(streampool.CName).(streampool.StreamPool)
	return synctestproto.DRPCRegisterCounterSync(serv, r)
}

func (r *RpcServer) Name() (name string) {
	return RpcName
}
