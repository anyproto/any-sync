package requesthandler

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/client"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/syncpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/treecache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

type requestHandler struct {
	treeCache treecache.Service
	client    client.Client
	account   account.Service
}

func NewRequestHandler() app.Component {
	return &requestHandler{}
}

type RequestHandler interface {
	HandleHeadUpdate(ctx context.Context, senderId string, update *syncpb.SyncHeadUpdate) (err error)
	HandleFullSyncRequest(ctx context.Context, senderId string, request *syncpb.SyncFullRequest) (err error)
	HandleFullSyncResponse(ctx context.Context, senderId string, request *syncpb.SyncFullRequest) (err error)
}

const CName = "SyncRequestHandler"

func (r *requestHandler) Init(ctx context.Context, a *app.App) (err error) {
	r.treeCache = a.MustComponent(treecache.CName).(treecache.Service)
	r.client = a.MustComponent(client.CName).(client.Client)
	r.account = a.MustComponent(account.CName).(account.Service)
	return nil
}

func (r *requestHandler) Name() (name string) {
	return CName
}

func (r *requestHandler) Run(ctx context.Context) (err error) {
	return nil
}

func (r *requestHandler) Close(ctx context.Context) (err error) {
	return nil
}

func (r *requestHandler) HandleHeadUpdate(ctx context.Context, senderId string, update *syncpb.SyncHeadUpdate) (err error) {
	var (
		fullRequest  *syncpb.SyncFullRequest
		snapshotPath []string
		result       acltree.AddResult
	)

	err = r.treeCache.Do(ctx, update.TreeId, func(tree acltree.ACLTree) error {
		// TODO: check if we already have those changes
		result, err = tree.AddRawChanges(ctx, update.Changes...)
		if err != nil {
			return err
		}
		shouldFullSync := !slice.UnsortedEquals(update.Heads, tree.Heads())
		snapshotPath = tree.SnapshotPath()
		if shouldFullSync {
			fullRequest, err = r.prepareFullSyncRequest(update.TreeId, update.TreeHeader, update.SnapshotPath, tree)
			if err != nil {
				return err
			}
		}
		return nil
	})
	// if there are no such tree
	if err == treestorage.ErrUnknownTreeId {
		// TODO: maybe we can optimize this by sending the header and stuff right away, so when the tree is created we are able to add it on first request
		fullRequest = &syncpb.SyncFullRequest{
			TreeId:     update.TreeId,
			TreeHeader: update.TreeHeader,
		}
	}
	// if we have incompatible heads, or we haven't seen the tree at all
	if fullRequest != nil {
		return r.client.RequestFullSync(senderId, fullRequest)
	}
	// if error or nothing has changed
	if err != nil || len(result.Added) == 0 {
		return err
	}
	// otherwise sending heads update message
	newUpdate := &syncpb.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
		TreeId:       update.TreeId,
		TreeHeader:   update.TreeHeader,
	}
	return r.client.NotifyHeadsChanged(newUpdate)
}

func (r *requestHandler) HandleFullSyncRequest(ctx context.Context, senderId string, request *syncpb.SyncFullRequest) (err error) {
	var (
		fullResponse *syncpb.SyncFullResponse
		snapshotPath []string
		result       acltree.AddResult
	)

	err = r.treeCache.Do(ctx, request.TreeId, func(tree acltree.ACLTree) error {
		// TODO: check if we already have those changes
		// if we have non-empty request
		if len(request.Heads) != 0 {
			result, err = tree.AddRawChanges(ctx, request.Changes...)
			if err != nil {
				return err
			}
		}
		snapshotPath = tree.SnapshotPath()
		fullResponse, err = r.prepareFullSyncResponse(request.TreeId, request.SnapshotPath, request.Changes, tree)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = r.client.SendFullSyncResponse(senderId, fullResponse)
	// if error or nothing has changed
	if err != nil || len(result.Added) == 0 {
		return err
	}

	// otherwise sending heads update message
	newUpdate := &syncpb.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
		TreeId:       request.TreeId,
		TreeHeader:   request.TreeHeader,
	}
	return r.client.NotifyHeadsChanged(newUpdate)
}

func (r *requestHandler) HandleFullSyncResponse(ctx context.Context, senderId string, response *syncpb.SyncFullResponse) (err error) {
	var (
		snapshotPath []string
		result       acltree.AddResult
	)

	err = r.treeCache.Do(ctx, response.TreeId, func(tree acltree.ACLTree) error {
		// TODO: check if we already have those changes
		result, err = tree.AddRawChanges(ctx, response.Changes...)
		if err != nil {
			return err
		}
		snapshotPath = tree.SnapshotPath()
		return nil
	})
	// if error or nothing has changed
	if (err != nil || len(result.Added) == 0) && err != treestorage.ErrUnknownTreeId {
		return err
	}
	// if we have a new tree
	if err == treestorage.ErrUnknownTreeId {
		err = r.createTree(ctx, response)
		if err != nil {
			return err
		}
	}
	// sending heads update message
	newUpdate := &syncpb.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
		TreeId:       response.TreeId,
	}
	return r.client.NotifyHeadsChanged(newUpdate)
}

func (r *requestHandler) prepareFullSyncRequest(treeId string, header *treepb.TreeHeader, theirPath []string, tree acltree.ACLTree) (*syncpb.SyncFullRequest, error) {
	ourChanges, err := tree.ChangesAfterCommonSnapshot(theirPath)
	if err != nil {
		return nil, err
	}
	return &syncpb.SyncFullRequest{
		Heads:        tree.Heads(),
		Changes:      ourChanges,
		TreeId:       treeId,
		SnapshotPath: tree.SnapshotPath(),
		TreeHeader:   header,
	}, nil
}

func (r *requestHandler) prepareFullSyncResponse(
	treeId string,
	theirPath []string,
	theirChanges []*aclpb.RawChange,
	tree acltree.ACLTree) (*syncpb.SyncFullResponse, error) {
	// TODO: we can probably use the common snapshot calculated on the request step from previous peer
	ourChanges, err := tree.ChangesAfterCommonSnapshot(theirPath)
	if err != nil {
		return nil, err
	}
	theirMap := make(map[string]struct{})
	for _, ch := range theirChanges {
		theirMap[ch.Id] = struct{}{}
	}

	// filtering our changes, so we will not send the same changes back
	var final []*aclpb.RawChange
	for _, ch := range ourChanges {
		if _, exists := theirMap[ch.Id]; exists {
			final = append(final, ch)
		}
	}

	return &syncpb.SyncFullResponse{
		Heads:        tree.Heads(),
		Changes:      final,
		TreeId:       treeId,
		SnapshotPath: tree.SnapshotPath(),
		TreeHeader:   tree.Header(),
	}, nil
}

func (r *requestHandler) createTree(ctx context.Context, response *syncpb.SyncFullResponse) error {
	return r.treeCache.Add(ctx, response.TreeId, response.TreeHeader, response.Changes)
}
