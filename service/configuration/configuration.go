package configuration

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
)

type Configuration interface {
	Id() string
	AllPeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error)
	OnePeer(ctx context.Context, spaceId string) (p peer.Peer, err error)
}
