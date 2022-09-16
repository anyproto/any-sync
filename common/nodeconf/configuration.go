package nodeconf

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-chash"
)

func New() Service {
	return new(service)
}

type Configuration interface {
	// Id returns current nodeconf id
	Id() string
	// AllPeers returns all peers by spaceId except current account
	AllPeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error)
	// OnePeer returns one of peer for spaceId
	OnePeer(ctx context.Context, spaceId string) (p peer.Peer, err error)
	// ResponsiblePeers returns peers for the space id that are responsible for the space
	ResponsiblePeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error)
	// NodeIds returns list of peerId for given spaceId
	NodeIds(spaceId string) []string
	// IsResponsible checks if current account responsible for given spaceId
	IsResponsible(spaceId string) bool
}

type configuration struct {
	id        string
	accountId string
	pool      pool.Pool
	chash     chash.CHash
}

func (c *configuration) Id() string {
	return c.id
}

func (c *configuration) AllPeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error) {
	nodeIds := c.NodeIds(spaceId)
	peers = make([]peer.Peer, 0, len(nodeIds))
	for _, id := range nodeIds {
		p, e := c.pool.Get(ctx, id)
		if e == nil {
			peers = append(peers, p)
		}
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("unable to connect to any node")
	}
	return
}

func (c *configuration) ResponsiblePeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error) {
	if c.IsResponsible(spaceId) {
		return c.AllPeers(ctx, spaceId)
	} else {
		var one peer.Peer
		one, err = c.OnePeer(ctx, spaceId)
		if err != nil {
			return
		}
		peers = []peer.Peer{one}
		return
	}
}

func (c *configuration) OnePeer(ctx context.Context, spaceId string) (p peer.Peer, err error) {
	nodeIds := c.NodeIds(spaceId)
	return c.pool.GetOneOf(ctx, nodeIds)
}

func (c *configuration) NodeIds(spaceId string) []string {
	members := c.chash.GetMembers(spaceId)
	res := make([]string, 0, len(members))
	for _, m := range members {
		if m.Id() != c.accountId {
			res = append(res, m.Id())
		}
	}
	return res
}

func (c *configuration) IsResponsible(spaceId string) bool {
	for _, m := range c.chash.GetMembers(spaceId) {
		if m.Id() == c.accountId {
			return true
		}
	}
	return false
}
