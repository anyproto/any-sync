package synctest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

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
	counter    *Counter
	streamPool streampool.StreamPool
	loop       periodicsync.PeriodicSync
	ownId      string
}

func (c *CounterGenerator) Init(a *app.App) (err error) {
	c.counter = a.MustComponent(CounterName).(*Counter)
	c.ownId = a.MustComponent(PeerName).(*PeerProvider).myPeer
	c.streamPool = a.MustComponent(streampool.CName).(streampool.StreamPool)
	c.loop = periodicsync.NewPeriodicSyncDuration(time.Millisecond*100, time.Millisecond*100, c.update, log)
	return
}

func (c *CounterGenerator) Name() (name string) {
	return CounterGeneratorName
}

func (c *CounterGenerator) update(ctx context.Context) error {
	res := c.counter.Generate()
	randChoice := rand.Int()%2 == 0
	if randChoice {
		fmt.Println("Broadcast", res, "by", c.ownId)
		return c.streamPool.Broadcast(ctx, &synctestproto.CounterIncrease{
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
