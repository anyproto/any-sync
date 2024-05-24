package net

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/internal/peerservice"
	"github.com/anyproto/any-sync/net/internal/pool"
	"github.com/anyproto/any-sync/net/internal/rpc/debugserver"
	"github.com/anyproto/any-sync/net/internal/rpc/server"
	"github.com/anyproto/any-sync/net/internal/secureservice"
	"github.com/anyproto/any-sync/net/internal/streampool"
	"github.com/anyproto/any-sync/net/internal/transport/quic"
	"github.com/anyproto/any-sync/net/internal/transport/yamux"
)

func NetworkApp() *app.App {
	a := &app.App{}
	a.RegisterPrivate(secureservice.New())
	a.RegisterPrivate(server.New())
	a.RegisterPrivate(debugserver.New())
	a.RegisterPrivate(pool.New())
	a.RegisterPrivate(peerservice.New())
	a.RegisterPrivate(yamux.New())
	a.RegisterPrivate(quic.New())
	a.RegisterPrivate(streampool.New())

	a.Register(New())
	return a
}
