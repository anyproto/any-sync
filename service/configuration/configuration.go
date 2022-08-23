package configuration

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-chash"
)

func New() Service {
	return new(service)
}

type Configuration interface {
	Id() string
	AllPeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error)
	OnePeer(ctx context.Context, spaceId string) (p peer.Peer, err error)
}

type configuration struct {
	id    string
	pool  pool.Pool
	chash chash.CHash
}

func (c *configuration) Id() string {
	return c.id
}

func (c *configuration) AllPeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *configuration) OnePeer(ctx context.Context, spaceId string) (p peer.Peer, err error) {
	//TODO implement me
	panic("implement me")
}
