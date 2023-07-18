package headsync

import (
	"context"
	"fmt"
	"time"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/util/slice"
	"go.uber.org/zap"
)

type DiffSyncer interface {
	Sync(ctx context.Context) error
	RemoveObjects(ids []string)
	UpdateHeads(id string, heads []string)
	Init()
	Close() error
}

func newDiffSyncer(hs *headSync) DiffSyncer {
	return &diffSyncer{
		diff:               hs.diff,
		spaceId:            hs.spaceId,
		treeManager:        hs.treeManager,
		storage:            hs.storage,
		peerManager:        hs.peerManager,
		clientFactory:      spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceSyncClient),
		credentialProvider: hs.credentialProvider,
		log:                log,
		syncStatus:         hs.syncStatus,
		deletionState:      hs.deletionState,
		syncAcl:            hs.syncAcl,
	}
}

type diffSyncer struct {
	spaceId            string
	diff               ldiff.Diff
	peerManager        peermanager.PeerManager
	treeManager        treemanager.TreeManager
	storage            spacestorage.SpaceStorage
	clientFactory      spacesyncproto.ClientFactory
	log                logger.CtxLogger
	deletionState      deletionstate.ObjectDeletionState
	credentialProvider credentialprovider.CredentialProvider
	syncStatus         syncstatus.StatusUpdater
	treeSyncer         treemanager.TreeSyncer
	syncAcl            syncacl.SyncAcl
}

func (d *diffSyncer) Init() {
	d.deletionState.AddObserver(d.RemoveObjects)
	d.treeSyncer = d.treeManager.NewTreeSyncer(d.spaceId, d.treeManager)
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
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		return
	}
	defer p.ReleaseDrpcConn(conn)

	var (
		cl           = d.clientFactory.Client(conn)
		rdiff        = NewRemoteDiff(d.spaceId, cl)
		stateCounter = d.syncStatus.StateCounter()
		syncAclId    = d.syncAcl.Id()
	)

	newIds, changedIds, removedIds, err := d.diff.Diff(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil && err != spacesyncproto.ErrSpaceMissing {
		if err == spacesyncproto.ErrSpaceIsDeleted {
			d.log.Debug("got space deleted while syncing")
			d.treeSyncer.SyncAll(ctx, p.Id(), []string{d.storage.SpaceSettingsId()}, nil)
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
	missingIds := d.deletionState.Filter(newIds)
	existingIds := append(d.deletionState.Filter(removedIds), d.deletionState.Filter(changedIds)...)
	d.syncStatus.RemoveAllExcept(p.Id(), existingIds, stateCounter)

	prevExistingLen := len(existingIds)
	existingIds = slice.DiscardFromSlice(existingIds, func(s string) bool {
		return s == syncAclId
	})
	// if we removed acl head from the list
	if len(existingIds) < prevExistingLen {
		if syncErr := d.syncAcl.SyncWithPeer(ctx, p.Id()); syncErr != nil {
			log.Warn("failed to send acl sync message to peer", zap.String("aclId", syncAclId))
		}
	}

	// treeSyncer should not get acl id, that's why we filter existing ids before
	err = d.treeSyncer.SyncAll(ctx, p.Id(), existingIds, missingIds)
	if err != nil {
		return err
	}
	d.log.Info("sync done:",
		zap.Int("newIds", len(newIds)),
		zap.Int("changedIds", len(changedIds)),
		zap.Int("removedIds", len(removedIds)),
		zap.Int("already deleted ids", totalLen-prevExistingLen-len(missingIds)),
		zap.String("peerId", p.Id()),
	)
	return
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

func (d *diffSyncer) Close() error {
	return d.treeSyncer.Close()
}
