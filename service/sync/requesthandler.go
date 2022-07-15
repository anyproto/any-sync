package sync

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/syncpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/treecache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

type requestHander struct {
	treeCache treecache.Service
	client    SyncClient
}

func (r *requestHander) HandleHeadUpdate(ctx context.Context, senderId string, update *syncpb.SyncHeadUpdate) (err error) {
	var fullRequest *syncpb.SyncFullRequest
	var snapshotPath []string
	var result acltree.AddResult
	defer func() {
		if err != nil || fullRequest != nil {
			return
		}
		newUpdate := &syncpb.SyncHeadUpdate{
			Heads:        result.Heads,
			Changes:      result.Added,
			SnapshotPath: snapshotPath,
			TreeId:       update.TreeId,
		}
		err = r.client.NotifyHeadsChanged(newUpdate)
	}()
	err = r.treeCache.Do(ctx, update.TreeId, func(tree acltree.ACLTree) error {
		// TODO: check if we already have those changes
		result, err = tree.AddRawChanges(ctx, update.Changes...)
		if err != nil {
			return err
		}
		shouldFullSync := !slice.UnsortedEquals(update.Heads, tree.Heads())
		snapshotPath = tree.SnapshotPath()
		if shouldFullSync {
			fullRequest, err = r.prepareFullSyncRequest(tree)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if fullRequest != nil {
		return r.client.RequestFullSync(senderId, fullRequest)
	}
	return
}

func (r *requestHander) HandleFullSync(ctx context.Context, senderId string, request *syncpb.SyncFullRequest) error {
	return nil
}

func (r *requestHander) prepareFullSyncRequest(tree acltree.ACLTree) (*syncpb.SyncFullRequest, error) {

	return nil, nil
}
