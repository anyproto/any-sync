package diffservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff"
	"go.uber.org/zap"
	"sync"
	"time"
)

type DiffSyncer interface {
	Sync(ctx context.Context) error
	RemoveObjects(ids []string)
	UpdateHeads(id string, heads []string)
}

func newDiffSyncer(
	spaceId string,
	diff ldiff.Diff,
	confConnector nodeconf.ConfConnector,
	cache treegetter.TreeGetter,
	storage storage.SpaceStorage,
	clientFactory spacesyncproto.ClientFactory,
	log *zap.Logger) DiffSyncer {
	return &diffSyncer{
		diff:          diff,
		spaceId:       spaceId,
		cache:         cache,
		storage:       storage,
		confConnector: confConnector,
		clientFactory: clientFactory,
		log:           log,
		removedIds:    map[string]struct{}{},
	}
}

type diffSyncer struct {
	sync.Mutex
	spaceId       string
	diff          ldiff.Diff
	confConnector nodeconf.ConfConnector
	cache         treegetter.TreeGetter
	storage       storage.SpaceStorage
	clientFactory spacesyncproto.ClientFactory
	log           *zap.Logger
	removedIds    map[string]struct{}
}

func (d *diffSyncer) RemoveObjects(ids []string) {
	d.Lock()
	defer d.Unlock()
	for _, id := range ids {
		d.diff.RemoveId(id)
		d.removedIds[id] = struct{}{}
	}
}

func (d *diffSyncer) UpdateHeads(id string, heads []string) {
	d.Lock()
	defer d.Unlock()
	if _, exists := d.removedIds[id]; exists {
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
	cl := d.clientFactory.Client(p)
	rdiff := remotediff.NewRemoteDiff(d.spaceId, cl)
	newIds, changedIds, removedIds, err := d.diff.Diff(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil && err != spacesyncproto.ErrSpaceMissing {
		return err
	}
	if err == spacesyncproto.ErrSpaceMissing {
		return d.sendPushSpaceRequest(ctx, cl)
	}
	var afterFilterIds []string
	filter := func(ids []string) {
		for _, id := range ids {
			if _, exists := d.removedIds[id]; !exists {
				afterFilterIds = append(afterFilterIds, id)
			}
		}
	}
	d.Lock()
	totalLen := 0
	// not syncing ids which were removed through settings document
	for _, ids := range [][]string{newIds, changedIds, removedIds} {
		totalLen += len(ids)
		filter(ids)
	}
	d.Unlock()

	ctx = peer.CtxWithPeerId(ctx, p.Id())
	d.pingTreesInCache(ctx, afterFilterIds)

	d.log.Info("sync done:", zap.Int("newIds", len(newIds)),
		zap.Int("changedIds", len(changedIds)),
		zap.Int("removedIds", len(removedIds)),
		zap.Int("filteredIds", totalLen-len(afterFilterIds)))
	return
}

func (d *diffSyncer) pingTreesInCache(ctx context.Context, trees []string) {
	for _, tId := range trees {
		_, _ = d.cache.GetTree(ctx, d.spaceId, tId)
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

	spaceSettingsTreeStorage, err := d.storage.TreeStorage(d.storage.SpaceSettingsId())
	if err != nil {
		return
	}
	spaceSettingsRoot, err := spaceSettingsTreeStorage.Root()
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
