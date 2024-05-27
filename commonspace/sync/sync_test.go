package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/synctest"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
)

var ctx = context.Background()

func TestNewSyncService(t *testing.T) {
	connProvider := synctest.NewConnProvider()
	var (
		firstApp  = &app.App{}
		secondApp = &app.App{}
	)
	firstApp.Register(connProvider).
		Register(rpctest.NewTestServer()).
		Register(synctest.NewRpcServer()).
		Register(synctest.NewPeerProvider("first"))
	secondApp.Register(connProvider).
		Register(rpctest.NewTestServer()).
		Register(synctest.NewRpcServer()).
		Register(synctest.NewPeerProvider("second"))
	require.NoError(t, firstApp.Start(ctx))
	require.NoError(t, secondApp.Start(ctx))
	pr1, err := firstApp.Component(synctest.PeerName).(*synctest.PeerProvider).GetPeer("second")
	require.NoError(t, err)
	err = pr1.DoDrpc(ctx, func(conn drpc.Conn) error {
		cl := synctestproto.NewDRPCCounterSyncClient(conn)
		_, err := cl.CounterStreamRequest(ctx, &synctestproto.CounterRequest{ObjectId: "test1"})
		require.NoError(t, err)
		return nil
	})
	pr2, err := secondApp.Component(synctest.PeerName).(*synctest.PeerProvider).GetPeer("first")
	require.NoError(t, err)
	err = pr2.DoDrpc(ctx, func(conn drpc.Conn) error {
		cl := synctestproto.NewDRPCCounterSyncClient(conn)
		_, err := cl.CounterStreamRequest(ctx, &synctestproto.CounterRequest{ObjectId: "test2"})
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)
}
