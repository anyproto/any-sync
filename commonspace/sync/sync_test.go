package sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/synctest"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/streampool"
)

var ctx = context.Background()

func TestNewSyncService(t *testing.T) {
	connProvider := synctest.NewConnProvider([]string{"first", "second"})
	var (
		firstApp  = newFixture(t, "first", counterFixtureParams{connProvider: connProvider, start: 0, delta: 2})
		secondApp = newFixture(t, "second", counterFixtureParams{connProvider: connProvider, start: 1, delta: 2})
	)
	require.NoError(t, firstApp.a.Start(ctx))
	require.NoError(t, secondApp.a.Start(ctx))
	time.Sleep(100 * time.Second)
	firstApp.a.Close(context.Background())
	secondApp.a.Close(context.Background())
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
		Register(synctest.NewConfig()).
		Register(rpctest.NewTestServer()).
		Register(synctest.NewCounterStreamOpener()).
		Register(synctest.NewPeerProvider(peerId)).
		Register(synctest.NewCounter(params.start, params.delta)).
		Register(streampool.NewStreamPool()).
		Register(synctest.NewCounterSyncDepsFactory()).
		Register(NewSyncService()).
		Register(synctest.NewCounterGenerator()).
		Register(synctest.NewRpcServer())
	return &counterFixture{a: a}
}
