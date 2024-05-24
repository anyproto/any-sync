package rpctest

import (
	"context"

	"github.com/anyproto/any-sync/net/internal/transport"
	peer2 "github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest/multiconntest"
)

func MultiConnPair(peerIdServ, peerIdClient string) (serv, client transport.MultiConn) {
	return multiconntest.MultiConnPair(peer2.CtxWithPeerId(context.Background(), peerIdServ), peer2.CtxWithPeerId(context.Background(), peerIdClient))
}
