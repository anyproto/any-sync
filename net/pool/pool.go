package pool

import (
	"context"
	"errors"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/app/ocache"
	"github.com/anytypeio/any-sync/net/dialer"
	"github.com/anytypeio/any-sync/net/peer"
	"math/rand"
	"time"
)

const (
	CName = "common.net.pool"
)

var log = logger.NewNamed(CName)

var (
	ErrUnableToConnect = errors.New("unable to connect")
)

func New() Pool {
	return &pool{}
}

// Pool creates and caches outgoing connection
type Pool interface {
	// Get lookups to peer in existing connections or creates and cache new one
	Get(ctx context.Context, id string) (peer.Peer, error)
	// Dial creates new connection to peer and not use cache
	Dial(ctx context.Context, id string) (peer.Peer, error)
	// GetOneOf searches at least one existing connection in cache or creates a new one from a randomly selected id from given list
	GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error)

	DialOneOf(ctx context.Context, peerIds []string) (peer.Peer, error)

	app.ComponentRunnable
}

type pool struct {
	cache  ocache.OCache
	dialer dialer.Dialer
}

func (p *pool) Init(a *app.App) (err error) {
	p.dialer = a.MustComponent(dialer.CName).(dialer.Dialer)
	p.cache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return p.dialer.Dial(ctx, id)
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Minute*5),
	)
	return nil
}

func (p *pool) Name() (name string) {
	return CName
}

func (p *pool) Run(ctx context.Context) (err error) {
	return nil
}

func (p *pool) Get(ctx context.Context, id string) (peer.Peer, error) {
	v, err := p.cache.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	pr := v.(peer.Peer)
	select {
	case <-pr.Closed():
	default:
		return pr, nil
	}
	p.cache.Remove(id)
	return p.Get(ctx, id)
}

func (p *pool) Dial(ctx context.Context, id string) (peer.Peer, error) {
	return p.dialer.Dial(ctx, id)
}

func (p *pool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	// finding existing connection
	for _, peerId := range peerIds {
		if v, err := p.cache.Pick(ctx, peerId); err == nil {
			pr := v.(peer.Peer)
			select {
			case <-pr.Closed():
			default:
				return pr, nil
			}
		}
	}
	// shuffle ids for better consistency
	rand.Shuffle(len(peerIds), func(i, j int) {
		peerIds[i], peerIds[j] = peerIds[j], peerIds[i]
	})
	// connecting
	for _, peerId := range peerIds {
		if v, err := p.cache.Get(ctx, peerId); err == nil {
			return v.(peer.Peer), nil
		}
	}
	return nil, ErrUnableToConnect
}

func (p *pool) DialOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	// shuffle ids for better consistency
	rand.Shuffle(len(peerIds), func(i, j int) {
		peerIds[i], peerIds[j] = peerIds[j], peerIds[i]
	})
	// connecting
	for _, peerId := range peerIds {
		if v, err := p.dialer.Dial(ctx, peerId); err == nil {
			return v.(peer.Peer), nil
		}
	}
	return nil, ErrUnableToConnect
}

func (p *pool) Close(ctx context.Context) (err error) {
	return p.cache.Close()
}
