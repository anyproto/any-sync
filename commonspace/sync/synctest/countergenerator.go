package synctest

import (
	"context"
	"math/rand/v2"
	"time"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/periodicsync"
)

var log = logger.NewNamed(syncdeps.CName)

const CounterGeneratorName = "countergenerator"

type CounterGenerator struct {
	counter      *Counter
	streamPool   streampool.StreamPool
	connProvider *ConnProvider
	peerProvider *PeerProvider
	syncSender   syncSender
	loop         periodicsync.PeriodicSync
	ownId        string
}

func NewCounterGenerator() *CounterGenerator {
	return &CounterGenerator{}
}

type syncSender interface {
	BroadcastMessage(ctx context.Context, msg drpc.Message) error
}

func (c *CounterGenerator) Init(a *app.App) (err error) {
	c.counter = a.MustComponent(CounterName).(*Counter)
	c.peerProvider = a.MustComponent(PeerName).(*PeerProvider)
	c.ownId = c.peerProvider.myPeer
	c.syncSender = a.MustComponent("common.commonspace.sync").(syncSender)
	c.loop = periodicsync.NewPeriodicSyncDuration(time.Millisecond*100, 0, c.update, log)
	return
}

func (c *CounterGenerator) Name() (name string) {
	return CounterGeneratorName
}

func (c *CounterGenerator) update(ctx context.Context) error {
	res := c.counter.Generate()
	randChoice := rand.Int()%2 == 0
	if randChoice {
		return c.syncSender.BroadcastMessage(ctx, &synctestproto.CounterIncrease{
			Value:    res,
			ObjectId: "counter",
		})
	}
	return nil
}

func (c *CounterGenerator) Run(ctx context.Context) (err error) {
	c.loop.Run()
	return nil
}

func (c *CounterGenerator) Close(ctx context.Context) (err error) {
	c.loop.Close()
	return nil
}
