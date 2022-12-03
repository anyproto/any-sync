package nodeconf

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
)

type ConfConnector interface {
	Configuration() Configuration
	GetResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error)
	DialResponsiblePeers(ctx context.Context, spaceId string, activeNodeIds []string) ([]peer.Peer, error)
}

type confConnector struct {
	conf Configuration
	pool pool.Pool
}

func NewConfConnector(conf Configuration, pool pool.Pool) ConfConnector {
	return &confConnector{conf: conf, pool: pool}
}

func (s *confConnector) Configuration() Configuration {
	return s.conf
}

func (s *confConnector) GetResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error) {
	return s.connectOneOrMany(ctx, spaceId, nil, s.pool.Get, s.pool.GetOneOf)
}

func (s *confConnector) DialResponsiblePeers(ctx context.Context, spaceId string, activeNodeIds []string) ([]peer.Peer, error) {
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
		p, err = connectOneOf(ctx, allNodes)
		if err != nil {
			return
		}
		peers = []peer.Peer{p}
	}
	return
}
