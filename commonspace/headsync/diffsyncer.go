package headsync

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"go.uber.org/zap"
	"time"
)

type DiffSyncer interface {
	Sync(ctx context.Context) error
	RemoveObjects(ids []string)
	UpdateHeads(id string, heads []string)
	Init(deletionState settingsstate.ObjectDeletionState)
}

func newDiffSyncer(
	spaceId string,
	diff ldiff.Diff,
	peerManager peermanager.PeerManager,
	cache treemanager.TreeManager,
	storage spacestorage.SpaceStorage,
	clientFactory spacesyncproto.ClientFactory,
	syncStatus syncstatus.StatusUpdater,
	credentialProvider credentialprovider.CredentialProvider,
	log logger.CtxLogger) DiffSyncer {
	return &diffSyncer{
		diff:               diff,
		spaceId:            spaceId,
		cache:              cache,
		storage:            storage,
		peerManager:        peerManager,
		clientFactory:      clientFactory,
		credentialProvider: credentialProvider,
		log:                log,
		syncStatus:         syncStatus,
	}
}

type diffSyncer struct {
	spaceId            string
	diff               ldiff.Diff
	peerManager        peermanager.PeerManager
	cache              treemanager.TreeManager
	storage            spacestorage.SpaceStorage
	clientFactory      spacesyncproto.ClientFactory
	log                logger.CtxLogger
	deletionState      settingsstate.ObjectDeletionState
	credentialProvider credentialprovider.CredentialProvider
	syncStatus         syncstatus.StatusUpdater
}

func (d *diffSyncer) Init(deletionState settingsstate.ObjectDeletionState) {
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
	// TODO: split diffsyncer into components
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
		if err == spacesyncproto.ErrSpaceIsDeleted {
			d.log.Debug("got space deleted while syncing")
			d.syncTrees(ctx, p.Id(), []string{d.storage.SpaceSettingsId()})
		}
		d.syncStatus.SetNodesOnline(p.Id(), false)
		return fmt.Errorf("diff error: %v", err)
	}
	d.syncStatus.SetNodesOnline(p.Id(), true)

	if err == spacesyncproto.ErrSpaceMissing {
		return d.sendPushSpaceRequest(ctx, p.Id(), cl)
	}

	totalLen := len(newIds) + len(changedIds) + len(removedIds)
	// not syncing ids which were removed through settings document
	filteredIds := d.deletionState.FilterJoin(newIds, changedIds, removedIds)

	d.syncStatus.RemoveAllExcept(p.Id(), filteredIds, stateCounter)

	d.syncTrees(ctx, p.Id(), filteredIds)

	d.log.Info("sync done:", zap.Int("newIds", len(newIds)),
		zap.Int("changedIds", len(changedIds)),
		zap.Int("removedIds", len(removedIds)),
		zap.Int("already deleted ids", totalLen-len(filteredIds)),
		zap.String("peerId", p.Id()),
	)
	return
}

func (d *diffSyncer) syncTrees(ctx context.Context, peerId string, trees []string) {
	for _, tId := range trees {
		log := d.log.With(zap.String("treeId", tId))
		tree, err := d.cache.GetTree(ctx, d.spaceId, tId)
		if err != nil {
			log.WarnCtx(ctx, "can't load tree", zap.Error(err))
			continue
		}
		syncTree, ok := tree.(synctree.SyncTree)
		if !ok {
			log.WarnCtx(ctx, "not a sync tree")
			continue
		}
		if err = syncTree.SyncWithPeer(ctx, peerId); err != nil {
			log.WarnCtx(ctx, "synctree.SyncWithPeer error", zap.Error(err))
		} else {
			log.DebugCtx(ctx, "success synctree.SyncWithPeer")
		}
	}
}

func (d *diffSyncer) sendPushSpaceRequest(ctx context.Context, peerId string, cl spacesyncproto.DRPCSpaceSyncClient) (err error) {
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

	cred, err := d.credentialProvider.GetCredential(ctx, header)
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
		Payload:    spacePayload,
		Credential: cred,
	})
	if err != nil {
		return
	}
	if e := d.subscribe(ctx, peerId); e != nil {
		d.log.WarnCtx(ctx, "error subscribing for space", zap.Error(e))
	}
	return
}

func (d *diffSyncer) subscribe(ctx context.Context, peerId string) (err error) {
	var msg = &spacesyncproto.SpaceSubscription{
		SpaceIds: []string{d.spaceId},
		Action:   spacesyncproto.SpaceSubscriptionAction_Subscribe,
	}
	payload, err := msg.Marshal()
	if err != nil {
		return
	}
	return d.peerManager.SendPeer(ctx, peerId, &spacesyncproto.ObjectSyncMessage{
		Payload: payload,
	})
}
