package synctest

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
	"github.com/anyproto/any-sync/net/peer"
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
	loop         periodicsync.PeriodicSync
	ownId        string
}

func NewCounterGenerator() *CounterGenerator {
	return &CounterGenerator{}
}

func (c *CounterGenerator) Init(a *app.App) (err error) {
	c.counter = a.MustComponent(CounterName).(*Counter)
	c.peerProvider = a.MustComponent(PeerName).(*PeerProvider)
	c.ownId = c.peerProvider.myPeer
	c.streamPool = a.MustComponent(streampool.CName).(streampool.StreamPool)
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
		err := c.streamPool.Send(ctx, &synctestproto.CounterIncrease{
			Value:    res,
			ObjectId: "counter",
		}, func(ctx context.Context) (peers []peer.Peer, err error) {
			for _, peerId := range c.peerProvider.GetPeerIds() {
				if peerId == c.ownId {
					continue
				}
				pr, err := c.peerProvider.GetPeer(peerId)
				if err != nil {
					return nil, err
				}
				peers = append(peers, pr)
			}
			return
		})
		if err != nil {
			return err
		}
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
