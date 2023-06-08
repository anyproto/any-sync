package rpctest

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/server"
	"net"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
)

type mockCtrl struct {
}

func (m mockCtrl) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	return nil
}

func (m mockCtrl) DrpcConfig() rpc.Config {
	return rpc.Config{}
}

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

func (s *TestServer) Init(a *app.App) (err error) {
	return nil
}

func (s *TestServer) Name() (name string) {
	return server.CName
}

func (s *TestServer) Run(ctx context.Context) (err error) {
	return nil
}

func (s *TestServer) Close(ctx context.Context) (err error) {
	return nil
}

func (s *TestServer) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	return s.Server.ServeOne(ctx, conn)
}

func (s *TestServer) DrpcConfig() rpc.Config {
	return rpc.Config{Stream: rpc.StreamConfig{MaxMsgSizeMb: 10}}
}

func (s *TestServer) Dial(peerId string) (clientPeer peer.Peer, err error) {
	mcS, mcC := MultiConnPair(peerId+"server", peerId)
	// NewPeer runs the accept loop
	_, err = peer.NewPeer(mcS, s)
	if err != nil {
		return
	}
	// and we ourselves don't call server methods on accept
	return peer.NewPeer(mcC, mockCtrl{})
}
