//go:generate mockgen -destination mock_treesyncer/mock_treesyncer.go github.com/anyproto/any-sync/commonspace/object/treesyncer TreeSyncer
package treesyncer

import (
	"context"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
)

const CName = "common.object.treesyncer"

type TreeSyncer interface {
	app.ComponentRunnable
	StartSync()
	StopSync()
	ShouldSync(peerId string) bool
	SyncAll(ctx context.Context, p peer.Peer, existing, missing []string) error
}
