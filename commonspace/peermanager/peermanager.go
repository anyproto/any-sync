//go:generate mockgen -destination mock_peermanager/mock_peermanager.go github.com/anyproto/any-sync/commonspace/peermanager PeerManager
package peermanager

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
)

const CName = "common.commonspace.peermanager"

type PeerManager interface {
	app.Component
	// SendPeer sends a message to a stream by peerId
	SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error)
	// Broadcast sends a message to all subscribed peers
	Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error)
	// GetResponsiblePeers dials or gets from cache responsible peers to unary operations
	GetResponsiblePeers(ctx context.Context) (peers []peer.Peer, err error)
}

type PeerManagerProvider interface {
	app.Component
	NewPeerManager(ctx context.Context, spaceId string) (sm PeerManager, err error)
}
