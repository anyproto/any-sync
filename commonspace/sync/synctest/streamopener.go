package synctest

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool"
)

func NewCounterStreamOpener() streampool.StreamOpener {
	return &CounterStreamOpener{}
}

type CounterStreamOpener struct {
}

func (c *CounterStreamOpener) Init(a *app.App) (err error) {
	return nil
}

func (c *CounterStreamOpener) Name() (name string) {
	return streampool.StreamOpenerCName
}

func (c *CounterStreamOpener) OpenStream(ctx context.Context, p peer.Peer) (stream drpc.Stream, tags []string, err error) {
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		return
	}
	objectStream, err := synctestproto.NewDRPCCounterSyncClient(conn).CounterStream(ctx)
	if err != nil {
		return
	}
	return objectStream, nil, nil
}
