package clientspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncstatus"
	"go.uber.org/zap"
)

type statusReceiver struct {
}

func (s *statusReceiver) UpdateTree(ctx context.Context, treeId string, status syncstatus.SyncStatus) (err error) {
	log.With(zap.String("treeId", treeId), zap.Bool("synced", status == syncstatus.SyncStatusSynced)).
		Debug("updating sync status")
	return nil
}

func (s *statusReceiver) UpdateNodeConnection(online bool) {
	log.With(zap.Bool("nodes online", online)).Debug("updating node connection")
}
