//go:generate mockgen -destination mock_confconnector/mock_confconnector.go github.com/anytypeio/any-sync/commonspace/confconnector ConfConnector
package confconnector

import (
	"context"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/pool"
	"github.com/anytypeio/any-sync/nodeconf"
	"golang.org/x/exp/slices"
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
	return s.conf
}

func (s *confConnector) Pool() pool.Pool {
	return s.pool
}

func (s *confConnector) GetResponsiblePeers(ctx context.Context, spaceId string) ([]peer.Peer, error) {
	return s.connectOneOrMany(ctx, spaceId, nil, s.pool.Get)
}

func (s *confConnector) DialInactiveResponsiblePeers(ctx context.Context, spaceId string, activeNodeIds []string) ([]peer.Peer, error) {
	return s.connectOneOrMany(ctx, spaceId, activeNodeIds, s.pool.Dial)
}

func (s *confConnector) connectOneOrMany(
	ctx context.Context,
	spaceId string,
	activeNodeIds []string,
	connectOne func(context.Context, string) (peer.Peer, error)) (peers []peer.Peer, err error) {
	var (
		inactiveNodeIds []string
		allNodes        = s.conf.NodeIds(spaceId)
	)
	for _, id := range allNodes {
		if !slices.Contains(activeNodeIds, id) {
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
