package rpctest

import (
	"context"
	"github.com/anyproto/any-sync/net/internal/peer"
	"github.com/anyproto/any-sync/net/internal/rpc/rpctest/multiconntest"
	"github.com/anyproto/any-sync/net/internal/transport"
)

func MultiConnPair(peerIdServ, peerIdClient string) (serv, client transport.MultiConn) {
	return multiconntest.MultiConnPair(peer.CtxWithPeerId(context.Background(), peerIdServ), peer.CtxWithPeerId(context.Background(), peerIdClient))
}
