package rpctest

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/server"
	"net"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
)

func NewTestServer() *TestServer {
	ts := &TestServer{
		Mux: drpcmux.New(),
	}
	ts.Server = drpcserver.New(ts.Mux)
	return ts
}

type TestServer struct {
	*drpcmux.Mux
	*drpcserver.Server
}

func (ts *TestServer) Init(a *app.App) (err error) {
	return nil
}

func (ts *TestServer) Name() (name string) {
	return server.CName
}

func (ts *TestServer) Run(ctx context.Context) (err error) {
	return nil
}

func (ts *TestServer) Close(ctx context.Context) (err error) {
	return nil
}

func (s *TestServer) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	return s.Server.ServeOne(ctx, conn)
}

func (s *TestServer) DrpcConfig() rpc.Config {
	return rpc.Config{Stream: rpc.StreamConfig{MaxMsgSizeMb: 10}}
}
