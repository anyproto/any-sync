package synctest

import (
	"context"
	"fmt"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool"
)

type CounterPeerManager struct {
	peerProvider *PeerProvider
	ownId        string
}

func NewCounterPeerManager() *CounterPeerManager {
	return &CounterPeerManager{}
}

func (c *CounterPeerManager) Init(a *app.App) (err error) {
	c.peerProvider = a.MustComponent(PeerName).(*PeerProvider)
	c.ownId = c.peerProvider.myPeer
	return
}

func (c *CounterPeerManager) Name() (name string) {
	return peermanager.CName
}

func (c *CounterPeerManager) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
	for _, peerId := range c.peerProvider.GetPeerIds() {
		if peerId == c.ownId {
			continue
		}
		pr, err := c.peerProvider.GetPeer(peerId)
		if err != nil {
			return nil, err
		}
		peers = append(peers, pr)
	}
	return
}

func (c *CounterPeerManager) GetNodePeers(ctx context.Context) (peers []peer.Peer, err error) {
	return nil, fmt.Errorf("unavailable")
}

func (c *CounterPeerManager) BroadcastMessage(ctx context.Context, msg drpc.Message, streamPool streampool.StreamPool) error {
	return streamPool.Send(ctx, msg, c.GetResponsiblePeers)
}
