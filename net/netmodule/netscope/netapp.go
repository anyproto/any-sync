package netscope

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/netmodule"
	"github.com/anyproto/any-sync/net/peerservice"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/debugserver"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/net/transport/yamux"
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

	a.Register(netmodule.New())
	return a
}
