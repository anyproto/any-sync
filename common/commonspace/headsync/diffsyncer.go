package headsync

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings/deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncstatus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff"
	"go.uber.org/zap"
	"time"
)

type DiffSyncer interface {
	Sync(ctx context.Context) error
	RemoveObjects(ids []string)
	UpdateHeads(id string, heads []string)
	Init(deletionState deletionstate.DeletionState)
}

func newDiffSyncer(
	spaceId string,
	diff ldiff.Diff,
	confConnector nodeconf.ConfConnector,
	cache treegetter.TreeGetter,
	storage storage.SpaceStorage,
	clientFactory spacesyncproto.ClientFactory,
	syncStatus syncstatus.SyncStatusUpdater,
	log *zap.Logger) DiffSyncer {
	return &diffSyncer{
		diff:          diff,
		spaceId:       spaceId,
		cache:         cache,
		storage:       storage,
		confConnector: confConnector,
		clientFactory: clientFactory,
		log:           log,
		syncStatus:    syncStatus,
	}
}

type diffSyncer struct {
	spaceId       string
	diff          ldiff.Diff
	confConnector nodeconf.ConfConnector
	cache         treegetter.TreeGetter
	storage       storage.SpaceStorage
	clientFactory spacesyncproto.ClientFactory
	log           *zap.Logger
	deletionState deletionstate.DeletionState
	syncStatus    syncstatus.SyncStatusUpdater
}

func (d *diffSyncer) Init(deletionState deletionstate.DeletionState) {
	d.deletionState = deletionState
	d.deletionState.AddObserver(d.RemoveObjects)
}

func (d *diffSyncer) RemoveObjects(ids []string) {
	for _, id := range ids {
		d.diff.RemoveId(id)
	}
}

func (d *diffSyncer) UpdateHeads(id string, heads []string) {
	if d.deletionState.Exists(id) {
		return
	}
	d.diff.Set(ldiff.Element{
		Id:   id,
		Head: concatStrings(heads),
	})
}

func (d *diffSyncer) Sync(ctx context.Context) error {
	st := time.Now()
	// diffing with responsible peers according to configuration
	peers, err := d.confConnector.GetResponsiblePeers(ctx, d.spaceId)
	if err != nil {
		return err
	}
	for _, p := range peers {
		if err := d.syncWithPeer(ctx, p); err != nil {
			d.log.Error("can't sync with peer", zap.String("peer", p.Id()), zap.Error(err))
		}
	}
	d.log.Info("synced", zap.String("spaceId", d.spaceId), zap.Duration("dur", time.Since(st)))
	return nil
}

func (d *diffSyncer) syncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	var (
		cl           = d.clientFactory.Client(p)
		rdiff        = NewRemoteDiff(d.spaceId, cl)
		stateCounter = d.syncStatus.StateCounter()
	)

	newIds, changedIds, removedIds, err := d.diff.Diff(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil && err != spacesyncproto.ErrSpaceMissing {
		d.syncStatus.SetNodesOnline(p.Id(), false)
		return err
	}
	d.syncStatus.SetNodesOnline(p.Id(), true)

	if err == spacesyncproto.ErrSpaceMissing {
		return d.sendPushSpaceRequest(ctx, cl)
	}

	totalLen := len(newIds) + len(changedIds) + len(removedIds)
	// not syncing ids which were removed through settings document
	filteredIds := d.deletionState.FilterJoin(newIds, changedIds, removedIds)

	d.syncStatus.RemoveAllExcept(p.Id(), filteredIds, stateCounter)

	ctx = peer.CtxWithPeerId(ctx, p.Id())
	d.pingTreesInCache(ctx, filteredIds)

	d.log.Info("sync done:", zap.Int("newIds", len(newIds)),
		zap.Int("changedIds", len(changedIds)),
		zap.Int("removedIds", len(removedIds)),
		zap.Int("already deleted ids", totalLen-len(filteredIds)))
	return
}

func (d *diffSyncer) pingTreesInCache(ctx context.Context, trees []string) {
	for _, tId := range trees {
		tree, err := d.cache.GetTree(ctx, d.spaceId, tId)
		if err != nil {
			continue
		}
		syncTree, ok := tree.(synctree.SyncTree)
		if !ok {
			continue
		}
		// the idea why we call it directly is that if we try to get it from cache
		// it may be already there (i.e. loaded)
		// and build func will not be called, thus we won't sync the tree
		// therefore we just do it manually
		syncTree.Ping()
	}
}

func (d *diffSyncer) sendPushSpaceRequest(ctx context.Context, cl spacesyncproto.DRPCSpaceClient) (err error) {
	aclStorage, err := d.storage.ACLStorage()
	if err != nil {
		return
	}

	root, err := aclStorage.Root()
	if err != nil {
		return
	}

	header, err := d.storage.SpaceHeader()
	if err != nil {
		return
	}

	settingsStorage, err := d.storage.TreeStorage(d.storage.SpaceSettingsId())
	if err != nil {
		return
	}
	spaceSettingsRoot, err := settingsStorage.Root()
	if err != nil {
		return
	}

	spacePayload := &spacesyncproto.SpacePayload{
		SpaceHeader:            header,
		AclPayload:             root.Payload,
		AclPayloadId:           root.Id,
		SpaceSettingsPayload:   spaceSettingsRoot.RawChange,
		SpaceSettingsPayloadId: spaceSettingsRoot.Id,
	}
	_, err = cl.PushSpace(ctx, &spacesyncproto.PushSpaceRequest{
		Payload: spacePayload,
	})
	return
}
