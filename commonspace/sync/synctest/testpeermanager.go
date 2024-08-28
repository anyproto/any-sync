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

type TestPeerManager struct {
	peerProvider *PeerProvider
	streamPool   streampool.StreamPool
	ownId        string
}

func NewTestPeerManager() *TestPeerManager {
	return &TestPeerManager{}
}

func (c *TestPeerManager) Init(a *app.App) (err error) {
	c.peerProvider = a.MustComponent(PeerName).(*PeerProvider)
	c.streamPool = a.MustComponent(streampool.CName).(streampool.StreamPool)
	c.ownId = c.peerProvider.myPeer
	return
}

func (c *TestPeerManager) Name() (name string) {
	return peermanager.CName
}

func (c *TestPeerManager) GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error) {
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

func (c *TestPeerManager) GetNodePeers(ctx context.Context) (peers []peer.Peer, err error) {
	return nil, fmt.Errorf("unavailable")
}

func (c *TestPeerManager) BroadcastMessage(ctx context.Context, msg drpc.Message) error {
	return c.streamPool.Send(ctx, msg, c.GetResponsiblePeers)
}

func (c *TestPeerManager) SendMessage(ctx context.Context, peerId string, msg drpc.Message) error {
	return c.streamPool.Send(ctx, msg, func(ctx context.Context) (peers []peer.Peer, err error) {
		pr, err := c.peerProvider.GetPeer(peerId)
		if err != nil {
			return nil, err
		}
		return []peer.Peer{pr}, nil
	})
}
