package confconnector

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
)

type ConfConnector interface {
	Configuration() nodeconf.Configuration
	Pool() pool.Pool
	GetResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error)
	DialInactiveResponsiblePeers(ctx context.Context, spaceId string, activeNodeIds []string) ([]peer.Peer, error)
}

type confConnector struct {
	conf nodeconf.Configuration
	pool pool.Pool
}

func NewConfConnector(conf nodeconf.Configuration, pool pool.Pool) ConfConnector {
	return &confConnector{conf: conf, pool: pool}
}

func (s *confConnector) Configuration() nodeconf.Configuration {
	// TODO: think about rewriting this, because these deps should not be exposed
	return s.conf
}

func (s *confConnector) Pool() pool.Pool {
	return s.pool
}

func (s *confConnector) GetResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error) {
	return s.connectOneOrMany(ctx, spaceId, nil, s.pool.Get, s.pool.GetOneOf)
}

func (s *confConnector) DialInactiveResponsiblePeers(ctx context.Context, spaceId string, activeNodeIds []string) ([]peer.Peer, error) {
	return s.connectOneOrMany(ctx, spaceId, activeNodeIds, s.pool.Dial, s.pool.DialOneOf)
}

func (s *confConnector) connectOneOrMany(
	ctx context.Context,
	spaceId string,
	activeNodeIds []string,
	connectOne func(context.Context, string) (peer.Peer, error),
	connectOneOf func(context.Context, []string) (peer.Peer, error)) (peers []peer.Peer, err error) {
	var (
		inactiveNodeIds []string
		allNodes        = s.conf.NodeIds(spaceId)
	)
	for _, id := range allNodes {
		if slice.FindPos(activeNodeIds, id) == -1 {
			inactiveNodeIds = append(inactiveNodeIds, id)
		}
	}

	if s.conf.IsResponsible(spaceId) {
		for _, id := range inactiveNodeIds {
			var p peer.Peer
			p, err = connectOne(ctx, id)
			if err != nil {
				continue
			}
			peers = append(peers, p)
		}
	} else if len(activeNodeIds) == 0 {
		// that means that all connected ids
		var p peer.Peer
		p, err = s.pool.GetOneOf(ctx, allNodes)
		if err != nil {
			return
		}

		// if we are dialling someone, we want to dial to the same peer which we cached
		// thus communication through streams and through diff will go to the same node
		p, err = connectOne(ctx, p.Id())
		if err != nil {
			return
		}
		peers = []peer.Peer{p}
	}
	return
}
