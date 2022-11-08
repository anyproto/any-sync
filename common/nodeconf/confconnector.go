package nodeconf

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
)

type ConfConnector interface {
	GetResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error)
	DialResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error)
}

type confConnector struct {
	conf Configuration
	pool pool.Pool
}

func NewConfConnector(conf Configuration, pool pool.Pool) ConfConnector {
	return &confConnector{conf: conf, pool: pool}
}

func (s *confConnector) GetResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error) {
	return s.connectOneOrMany(ctx, spaceId, s.pool.Get, s.pool.GetOneOf)
}

func (s *confConnector) DialResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error) {
	return s.connectOneOrMany(ctx, spaceId, s.pool.Dial, s.pool.DialOneOf)
}

func (s *confConnector) connectOneOrMany(
	ctx context.Context, spaceId string,
	connectOne func(context.Context, string) (peer.Peer, error),
	connectOneOf func(context.Context, []string) (peer.Peer, error)) (peers []peer.Peer, err error) {
	allNodes := s.conf.NodeIds(spaceId)
	if s.conf.IsResponsible(spaceId) {
		for _, id := range allNodes {
			var p peer.Peer
			p, err = connectOne(ctx, id)
			if err != nil {
				continue
			}
			peers = append(peers, p)
		}
	} else {
		var p peer.Peer
		p, err = connectOneOf(ctx, allNodes)
		if err != nil {
			return
		}
		peers = []peer.Peer{p}
	}
	return
}
