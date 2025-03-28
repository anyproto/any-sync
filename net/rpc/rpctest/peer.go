package rpctest

import (
	"context"
	"time"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest/multiconntest"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
)

func MultiConnPair(peerIdServ, peerIdClient string) (serv, client transport.MultiConn) {
	return multiconntest.MultiConnPair(
		peer.CtxWithProtoVersion(peer.CtxWithPeerId(context.Background(), peerIdServ), secureservice.ProtoVersion),
		peer.CtxWithProtoVersion(peer.CtxWithPeerId(context.Background(), peerIdClient), secureservice.ProtoVersion),
	)
}

func MultiConnPairWithIdentity(peerIdServ, peerIdClient string, serverIdentity []byte) (serv, client transport.MultiConn) {
	return multiconntest.MultiConnPair(
		peer.CtxWithProtoVersion(peer.CtxWithIdentity(peer.CtxWithPeerId(context.Background(), peerIdServ), serverIdentity), secureservice.ProtoVersion),
		peer.CtxWithProtoVersion(peer.CtxWithPeerId(context.Background(), peerIdClient), secureservice.ProtoVersion),
	)
}

type MockPeer struct {
	Ctx context.Context
}

func (m MockPeer) CloseChan() <-chan struct{} {
	return nil
}

func (m MockPeer) SetTTL(ttl time.Duration) {
}

func (m MockPeer) Id() string {
	return "peerId"
}

func (m MockPeer) Context() context.Context {
	if m.Ctx != nil {
		return m.Ctx
	}
	return context.Background()
}

func (m MockPeer) AcquireDrpcConn(ctx context.Context) (drpc.Conn, error) {
	return nil, nil
}

func (m MockPeer) ReleaseDrpcConn(conn drpc.Conn) {
	return
}

func (m MockPeer) DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error {
	return nil
}

func (m MockPeer) IsClosed() bool {
	return false
}

func (m MockPeer) TryClose(objectTTL time.Duration) (res bool, err error) {
	return false, err
}

func (m MockPeer) Close() (err error) {
	return nil
}
