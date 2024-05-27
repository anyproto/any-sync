package synctest

import (
	"fmt"

	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/rpc/server"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

const RpcName = "rpcserver"

type RpcServer struct {
}

func NewRpcServer() *RpcServer {
	return &RpcServer{}
}

func (r *RpcServer) CounterStreamRequest(request *synctestproto.CounterRequest, stream synctestproto.DRPCCounterSync_CounterStreamRequestStream) error {
	fmt.Println(request.ObjectId)
	return nil
}

func (r *RpcServer) CounterStream(request *synctestproto.CounterRequest, stream synctestproto.DRPCCounterSync_CounterStreamStream) error {
	return nil
}

func (r *RpcServer) Init(a *app.App) (err error) {
	serv := a.MustComponent(server.CName).(*rpctest.TestServer)
	return synctestproto.DRPCRegisterCounterSync(serv, r)
}

func (r *RpcServer) Name() (name string) {
	return RpcName
}
