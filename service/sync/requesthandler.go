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

func (r *requestHander) HandleHeadUpdate(ctx context.Context, senderId string, update *syncpb.SyncHeadUpdate) error {
	err := r.treeCache.Do(ctx, update.TreeId, func(tree acltree.ACLTree) error {
		_, err := tree.AddRawChanges(ctx, update.Changes...)
		if err != nil {
			return err
		}
		shouldFullSync := !r.compareHeads(update.Heads, tree.Heads())

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *requestHander) compareHeads(syncHeads []*syncpb.SyncHead, heads []string) bool {
	for _, head := range syncHeads {
		if slice.FindPos(heads, head.Id) == -1 {
			return false
		}
	}
	return true
}

func (r *requestHander) prepareFullSyncRequest(tree acltree.ACLTree) (*syncpb.SyncFullRequest, error) {
	
}
