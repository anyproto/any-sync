package headsync

import (
	"context"
	"errors"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/anyproto/any-sync/commonspace/object/treesyncer"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/util/slice"
)

type DiffSyncer interface {
	Sync(ctx context.Context) error
	RemoveObjects(ids []string)
	UpdateHeads(id string, heads []string)
	Init()
}

const logPeriodSecs = 200

func newDiffSyncer(hs *headSync) DiffSyncer {
	return &diffSyncer{
		diff:               hs.diff,
		spaceId:            hs.spaceId,
		storage:            hs.storage,
		peerManager:        hs.peerManager,
		clientFactory:      spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceSyncClient),
		credentialProvider: hs.credentialProvider,
		log:                newSyncLogger(hs.log, logPeriodSecs),
		deletionState:      hs.deletionState,
		syncAcl:            hs.syncAcl,
		treeSyncer:         hs.treeSyncer,
	}
}

type diffSyncer struct {
	spaceId            string
	diff               ldiff.Diff
	peerManager        peermanager.PeerManager
	treeManager        treemanager.TreeManager
	treeSyncer         treesyncer.TreeSyncer
	storage            spacestorage.SpaceStorage
	clientFactory      spacesyncproto.ClientFactory
	log                syncLogger
	deletionState      deletionstate.ObjectDeletionState
	credentialProvider credentialprovider.CredentialProvider
	syncAcl            syncacl.SyncAcl
}

func (d *diffSyncer) Init() {
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
		if err = d.syncWithPeer(peer.CtxWithPeerAddr(ctx, p.Id()), p); err != nil {
			if !errors.Is(err, &quic.IdleTimeoutError{}) && !errors.Is(err, context.DeadlineExceeded) {
				d.log.ErrorCtx(ctx, "can't sync with peer", zap.String("peer", p.Id()), zap.Error(err))
			}
		}
	}
	d.log.DebugCtx(ctx, "diff done", zap.String("spaceId", d.spaceId), zap.Duration("dur", time.Since(st)))
	return nil
}

func (d *diffSyncer) syncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	if !d.treeSyncer.ShouldSync(p.Id()) {
		return
	}
	ctx = logger.CtxWithFields(ctx, zap.String("peerId", p.Id()))
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		return
	}
	defer p.ReleaseDrpcConn(conn)

	var (
		cl                             = d.clientFactory.Client(conn)
		rdiff                          = NewRemoteDiff(d.spaceId, cl)
		syncAclId                      = d.syncAcl.Id()
		newIds, changedIds, removedIds []string
	)

	newIds, changedIds, removedIds, err = d.diff.Diff(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil {
		return d.onDiffError(ctx, p, cl, err)
	}
	totalLen := len(newIds) + len(changedIds) + len(removedIds)
	// not syncing ids which were removed through settings document
	missingIds := d.deletionState.Filter(newIds)
	existingIds := append(d.deletionState.Filter(removedIds), d.deletionState.Filter(changedIds)...)

	prevExistingLen := len(existingIds)
	existingIds = slice.DiscardFromSlice(existingIds, func(s string) bool {
		return s == syncAclId
	})

	// if we removed acl head from the list
	if len(existingIds) < prevExistingLen {
		if syncErr := d.syncAcl.SyncWithPeer(ctx, p); syncErr != nil {
			log.Warn("failed to send acl sync message to peer", zap.String("aclId", syncAclId))
		}
	}

	// treeSyncer should not get acl id, that's why we filter existing ids before
	err = d.treeSyncer.SyncAll(ctx, p, existingIds, missingIds)
	if err != nil {
		return err
	}
	d.log.logSyncDone(p.Id(), len(newIds), len(changedIds), len(removedIds), totalLen-len(existingIds)-len(missingIds))
	return
}

func (d *diffSyncer) onDiffError(ctx context.Context, p peer.Peer, cl spacesyncproto.DRPCSpaceSyncClient, err error) error {
	if err != spacesyncproto.ErrSpaceMissing {
		return err
	}
	// in case space is missing on peer, we should send push request
	err = d.sendPushSpaceRequest(ctx, p.Id(), cl)
	if err != nil {
		return err
	}
	return nil
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
		d.log.WarnCtx(ctx, "space push failed", zap.Error(err))
		return
	}
	d.log.InfoCtx(ctx, "space push completed successfully")
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
	return d.peerManager.SendMessage(ctx, peerId, &spacesyncproto.ObjectSyncMessage{
		Payload: payload,
	})
}
