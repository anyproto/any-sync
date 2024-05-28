//go:generate mockgen -destination mock_pool/mock_pool.go github.com/anyproto/any-sync/net/pool Pool
package pool

import (
	"context"
	"fmt"
	"math/rand"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
)

// Pool creates and caches outgoing connection
type Pool interface {
	// Get lookups to peer in existing connections or creates and outgoing new one
	Get(ctx context.Context, id string) (peer.Peer, error)
	// GetOneOf searches at least one existing connection in outgoing or creates a new one from a randomly selected id from given list
	GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error)
	// AddPeer adds incoming peer to the pool
	AddPeer(ctx context.Context, p peer.Peer) (err error)
	// Pick check if connection with peer exist without dial
	Pick(ctx context.Context, id string) (pr peer.Peer, err error)
}

type pool struct {
	outgoing ocache.OCache
	incoming ocache.OCache
}

func (p *pool) Name() (name string) {
	return CName
}

func (p *pool) Get(ctx context.Context, id string) (pr peer.Peer, err error) {
	// if we have incoming connection - try to reuse it
	if pr, err = p.get(ctx, p.incoming, id); err != nil {
		// or try to get or create outgoing
		return p.get(ctx, p.outgoing, id)
	}
	return
}

func (p *pool) get(ctx context.Context, source ocache.OCache, id string) (peer.Peer, error) {
	v, err := source.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	pr := v.(peer.Peer)
	if !pr.IsClosed() {
		return pr, nil
	}
	_, _ = source.Remove(ctx, id)
	return p.Get(ctx, id)
}

func (p *pool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	// finding existing connection
	for _, peerId := range peerIds {
		if v, err := p.incoming.Pick(ctx, peerId); err == nil {
			pr := v.(peer.Peer)
			if !pr.IsClosed() {
				return pr, nil
			}
			_, _ = p.incoming.Remove(ctx, peerId)
		}
		if v, err := p.outgoing.Pick(ctx, peerId); err == nil {
			pr := v.(peer.Peer)
			if !pr.IsClosed() {
				return pr, nil
			}
			_, _ = p.outgoing.Remove(ctx, peerId)
		}
	}
	// shuffle ids for better consistency
	indexes := make([]int, len(peerIds))
	for i := range indexes {
		indexes[i] = i
	}
	rand.Shuffle(len(indexes), func(i, j int) {
		indexes[i], indexes[j] = indexes[j], indexes[i]
	})
	// connecting
	var lastErr error
	for _, idx := range indexes {
		peerId := peerIds[idx]
		if v, err := p.Get(ctx, peerId); err == nil {
			return v, nil
		} else {
			log.Debug("unable to connect", zap.String("peerId", peerId), zap.Error(err))
			lastErr = err
		}
	}
	if _, ok := lastErr.(handshake.HandshakeError); !ok {
		lastErr = net.ErrUnableToConnect
	}
	return nil, lastErr
}

func (p *pool) AddPeer(ctx context.Context, pr peer.Peer) (err error) {
	if err = p.incoming.Add(pr.Id(), pr); err != nil {
		if err == ocache.ErrExists {
			// in case when an incoming connection with a peer already exists, we close and remove an existing connection
			if v, e := p.incoming.Pick(ctx, pr.Id()); e == nil {
				_ = v.Close()
				_, _ = p.incoming.Remove(ctx, pr.Id())
				return p.incoming.Add(pr.Id(), pr)
			}
		} else {
			return err
		}
	}
	return
}

func (p *pool) Pick(ctx context.Context, id string) (pr peer.Peer, err error) {
	// check if connection with peer exist without dial
	if pr, err = p.pick(ctx, p.incoming, id); err != nil {
		return p.pick(ctx, p.outgoing, id)
	}
	return
}

func (p *pool) pick(ctx context.Context, source ocache.OCache, id string) (peer.Peer, error) {
	v, err := source.Pick(ctx, id)
	if err != nil {
		return nil, err
	}
	pr := v.(peer.Peer)
	if !pr.IsClosed() {
		return pr, nil
	}
	return nil, fmt.Errorf("failed to pick connection with peer: peer not founc")
}
