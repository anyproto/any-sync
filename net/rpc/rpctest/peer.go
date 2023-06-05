package rpctest

import (
	"context"
	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
	yamux2 "github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/hashicorp/yamux"
	"net"
)

func MultiConnPair(peerIdServ, peerIdClient string) (serv, client transport.MultiConn) {
	sc, cc := net.Pipe()
	var servConn = make(chan transport.MultiConn, 1)
	go func() {
		sess, err := yamux.Server(sc, yamux.DefaultConfig())
		if err != nil {
			panic(err)
		}
		servConn <- yamux2.NewMultiConn(peer.CtxWithPeerId(context.Background(), peerIdServ), connutil.NewLastUsageConn(sc), "", sess)
	}()
	sess, err := yamux.Client(cc, yamux.DefaultConfig())
	if err != nil {
		panic(err)
	}
	client = yamux2.NewMultiConn(peer.CtxWithPeerId(context.Background(), peerIdClient), connutil.NewLastUsageConn(cc), "", sess)
	serv = <-servConn
	return
}
