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
	"github.com/anyproto/any-sync/net/streampool"
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
	//Register(synctest.NewCounterStreamOpener())
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

type counterFixture struct {
	a *app.App
}

type counterFixtureParams struct {
	connProvider *synctest.ConnProvider
	start        int32
	delta        int32
}

func newFixture(t *testing.T, peerId string, params counterFixtureParams) *counterFixture {
	a := &app.App{}
	a.Register(params.connProvider).
		Register(rpctest.NewTestServer()).
		Register(synctest.NewCounterStreamOpener()).
		Register(synctest.NewPeerProvider(peerId)).
		Register(synctest.NewCounter(params.start, params.delta)).
		Register(streampool.NewStreamPool()).
		Register(synctest.NewCounterSyncDepsFactory()).
		Register(NewSyncService()).
		//Register().
		Register(synctest.NewRpcServer())
	return nil
}
