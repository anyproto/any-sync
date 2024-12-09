//go:generate mockgen -destination mock_peermanager/mock_peermanager.go github.com/anyproto/any-sync/commonspace/peermanager PeerManager
package peermanager

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
)

const (
	CName = "common.commonspace.peermanager"
)

type PeerManager interface {
	app.Component
	// GetResponsiblePeers dials or gets from cache responsible peers
	GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error)
	// GetNodePeers dials or gets from cache node peers
	GetNodePeers(ctx context.Context) (peers []peer.Peer, err error)
	// BroadcastMessage sends message to all peers
	BroadcastMessage(ctx context.Context, msg drpc.Message) error
	// SendMessage sends message to peer
	SendMessage(ctx context.Context, peerId string, msg drpc.Message) error
	// KeepAlive sends keepAlive messages to needed peers
	KeepAlive(ctx context.Context)
}

type PeerManagerProvider interface {
	app.Component
	NewPeerManager(ctx context.Context, spaceId string) (sm PeerManager, err error)
}
