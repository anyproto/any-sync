package pool

import (
	"context"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/dialer"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"go.uber.org/zap"
	"math/rand"
)

// Pool creates and caches outgoing connection
type Pool interface {
	// Get lookups to peer in existing connections or creates and outgoing new one
	Get(ctx context.Context, id string) (peer.Peer, error)
	// GetOneOf searches at least one existing connection in outgoing or creates a new one from a randomly selected id from given list
	GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error)
	// AddPeer adds incoming peer to the pool
	AddPeer(ctx context.Context, p peer.Peer) (err error)
}

type pool struct {
	outgoing ocache.OCache
	incoming ocache.OCache
	dialer   dialer.Dialer
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
	select {
	case <-pr.Closed():
	default:
		return pr, nil
	}
	_, _ = source.Remove(ctx, id)
	return p.Get(ctx, id)
}

func (p *pool) Dial(ctx context.Context, id string) (peer.Peer, error) {
	return p.dialer.Dial(ctx, id)
}

func (p *pool) GetOneOf(ctx context.Context, peerIds []string) (peer.Peer, error) {
	// finding existing connection
	for _, peerId := range peerIds {
		if v, err := p.incoming.Pick(ctx, peerId); err == nil {
			pr := v.(peer.Peer)
			select {
			case <-pr.Closed():
			default:
				return pr, nil
			}
			_, _ = p.incoming.Remove(ctx, peerId)
		}
		if v, err := p.outgoing.Pick(ctx, peerId); err == nil {
			pr := v.(peer.Peer)
			select {
			case <-pr.Closed():
			default:
				return pr, nil
			}
			_, _ = p.outgoing.Remove(ctx, peerId)
		}
	}
	// shuffle ids for better consistency
	rand.Shuffle(len(peerIds), func(i, j int) {
		peerIds[i], peerIds[j] = peerIds[j], peerIds[i]
	})
	// connecting
	var lastErr error
	for _, peerId := range peerIds {
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
	return p.incoming.Add(pr.Id(), pr)
}
