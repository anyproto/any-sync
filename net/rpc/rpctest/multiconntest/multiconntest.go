package multiconntest

import (
	"context"
	"github.com/anyproto/any-sync/net/internal/connutil"
	"github.com/anyproto/any-sync/net/internal/transport"
	yamux2 "github.com/anyproto/any-sync/net/internal/transport/yamux"
	"github.com/hashicorp/yamux"
	"net"
)

func MultiConnPair(peerServCtx, peerClientCtx context.Context) (serv, client transport.MultiConn) {
	sc, cc := net.Pipe()
	var servConn = make(chan transport.MultiConn, 1)
	go func() {
		sess, err := yamux.Server(sc, yamux.DefaultConfig())
		if err != nil {
			panic(err)
		}
		servConn <- yamux2.NewMultiConn(peerServCtx, connutil.NewLastUsageConn(sc), "", sess)
	}()
	sess, err := yamux.Client(cc, yamux.DefaultConfig())
	if err != nil {
		panic(err)
	}
	client = yamux2.NewMultiConn(peerClientCtx, connutil.NewLastUsageConn(cc), "", sess)
	serv = <-servConn
	return
}
