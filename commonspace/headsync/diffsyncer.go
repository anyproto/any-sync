package headsync

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/app/ldiff"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/settings/deletionstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/rpc/rpcerr"
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
	peerManager peermanager.PeerManager,
	cache treegetter.TreeGetter,
	storage spacestorage.SpaceStorage,
	clientFactory spacesyncproto.ClientFactory,
	syncStatus syncstatus.StatusUpdater,
	log logger.CtxLogger) DiffSyncer {
	return &diffSyncer{
		diff:          diff,
		spaceId:       spaceId,
		cache:         cache,
		storage:       storage,
		peerManager:   peerManager,
		clientFactory: clientFactory,
		log:           log,
		syncStatus:    syncStatus,
	}
}

type diffSyncer struct {
	spaceId       string
	diff          ldiff.Diff
	peerManager   peermanager.PeerManager
	cache         treegetter.TreeGetter
	storage       spacestorage.SpaceStorage
	clientFactory spacesyncproto.ClientFactory
	log           logger.CtxLogger
	deletionState deletionstate.DeletionState
	syncStatus    syncstatus.StatusUpdater
}

func (d *diffSyncer) Init(deletionState deletionstate.DeletionState) {
	d.deletionState = deletionState
	d.deletionState.AddObserver(d.RemoveObjects)
}

func (d *diffSyncer) RemoveObjects(ids []string) {
	for _, id := range ids {
		_ = d.diff.RemoveId(id)
	}
	if err := d.storage.WriteSpaceHash(d.diff.Hash()); err != nil {
		d.log.Error("can't write space hash", zap.Error(err))
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
	if err := d.storage.WriteSpaceHash(d.diff.Hash()); err != nil {
		d.log.Error("can't write space hash", zap.Error(err))
	}
}

func (d *diffSyncer) Sync(ctx context.Context) error {
	st := time.Now()
	// diffing with responsible peers according to configuration
	peers, err := d.peerManager.GetResponsiblePeers(ctx)
	if err != nil {
		return err
	}
	var peerIds = make([]string, 0, len(peers))
	for _, p := range peers {
		peerIds = append(peerIds, p.Id())
	}
	d.log.DebugCtx(ctx, "start diffsync", zap.Strings("peerIds", peerIds))
	for _, p := range peers {
		if err = d.syncWithPeer(peer.CtxWithPeerId(ctx, p.Id()), p); err != nil {
			d.log.ErrorCtx(ctx, "can't sync with peer", zap.String("peer", p.Id()), zap.Error(err))
		}
	}
	d.log.InfoCtx(ctx, "diff done", zap.String("spaceId", d.spaceId), zap.Duration("dur", time.Since(st)))
	return nil
}

func (d *diffSyncer) syncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	ctx = logger.CtxWithFields(ctx, zap.String("peerId", p.Id()))
	var (
		cl           = d.clientFactory.Client(p)
		rdiff        = NewRemoteDiff(d.spaceId, cl)
		stateCounter = d.syncStatus.StateCounter()
	)

	newIds, changedIds, removedIds, err := d.diff.Diff(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil && err != spacesyncproto.ErrSpaceMissing {
		d.syncStatus.SetNodesOnline(p.Id(), false)
		return fmt.Errorf("diff error: %v", err)
	}
	d.syncStatus.SetNodesOnline(p.Id(), true)

	if err == spacesyncproto.ErrSpaceMissing {
		return d.sendPushSpaceRequest(ctx, cl)
	}

	totalLen := len(newIds) + len(changedIds) + len(removedIds)
	// not syncing ids which were removed through settings document
	filteredIds := d.deletionState.FilterJoin(newIds, changedIds, removedIds)

	d.syncStatus.RemoveAllExcept(p.Id(), filteredIds, stateCounter)

	d.pingTreesInCache(ctx, filteredIds)

	d.log.Info("sync done:", zap.Int("newIds", len(newIds)),
		zap.Int("changedIds", len(changedIds)),
		zap.Int("removedIds", len(removedIds)),
		zap.Int("already deleted ids", totalLen-len(filteredIds)),
		zap.String("peerId", p.Id()),
	)
	return
}

func (d *diffSyncer) pingTreesInCache(ctx context.Context, trees []string) {
	for _, tId := range trees {
		tree, err := d.cache.GetTree(ctx, d.spaceId, tId)
		if err != nil {
			d.log.InfoCtx(ctx, "can't load tree", zap.Error(err))
			continue
		}
		syncTree, ok := tree.(synctree.SyncTree)
		if !ok {
			d.log.InfoCtx(ctx, "not a sync tree", zap.String("objectId", tId))
			continue
		}
		// the idea why we call it directly is that if we try to get it from cache
		// it may be already there (i.e. loaded)
		// and build func will not be called, thus we won't sync the tree
		// therefore we just do it manually
		if err = syncTree.Ping(ctx); err != nil {
			d.log.WarnCtx(ctx, "synctree.Ping error", zap.Error(err), zap.String("treeId", tId))
		} else {
			d.log.DebugCtx(ctx, "success tree ping", zap.String("treeId", tId))
		}
	}
}

func (d *diffSyncer) sendPushSpaceRequest(ctx context.Context, cl spacesyncproto.DRPCSpaceSyncClient) (err error) {
	aclStorage, err := d.storage.AclStorage()
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
	_, err = cl.SpacePush(ctx, &spacesyncproto.SpacePushRequest{
		Payload: spacePayload,
	})
	return
}
