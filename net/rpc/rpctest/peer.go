package rpctest

import (
	"context"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest/multiconntest"
	"github.com/anyproto/any-sync/net/transport"
)

func MultiConnPair(peerIdServ, peerIdClient string) (serv, client transport.MultiConn) {
	return multiconntest.MultiConnPair(peer.CtxWithPeerId(context.Background(), peerIdServ), peer.CtxWithPeerId(context.Background(), peerIdClient))
}
