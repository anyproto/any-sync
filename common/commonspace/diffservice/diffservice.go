package diffservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ldiff"
	"go.uber.org/zap"
	"strings"
	"time"
)

type DiffService interface {
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	UpdateHeads(id string, heads []string)
	RemoveObject(id string)

	Init(objectIds []string)
	Close() (err error)
}

type diffService struct {
	spaceId      string
	periodicSync *periodicSync
	storage      storage.SpaceStorage
	nconf        nodeconf.Configuration
	diff         ldiff.Diff
	cache        cache.TreeCache
	log          *zap.Logger

	syncPeriod int
}

func NewDiffService(
	spaceId string,
	syncPeriod int,
	storage storage.SpaceStorage,
	nconf nodeconf.Configuration,
	cache cache.TreeCache,
	log *zap.Logger) DiffService {
	return &diffService{
		spaceId:    spaceId,
		storage:    storage,
		nconf:      nconf,
		cache:      cache,
		log:        log,
		syncPeriod: syncPeriod,
	}
}

func (d *diffService) Init(objectIds []string) {
	d.periodicSync = newPeriodicSync(d.syncPeriod, d.sync, d.log.With(zap.String("spaceId", d.spaceId)))
	d.diff = ldiff.New(16, 16)
	d.fillDiff(objectIds)
}

func (d *diffService) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	return remotediff.HandleRangeRequest(ctx, d.diff, req)
}

func (d *diffService) UpdateHeads(id string, heads []string) {
	d.diff.Set(ldiff.Element{
		Id:   id,
		Head: concatStrings(heads),
	})
}

func (d *diffService) RemoveObject(id string) {
	// TODO: add space document to remove ids
	d.diff.RemoveId(id)
}

func (d *diffService) Close() (err error) {
	d.periodicSync.Close()
	return nil
}

func (d *diffService) sync(ctx context.Context) error {
	st := time.Now()
	// diffing with responsible peers according to configuration
	peers, err := d.nconf.ResponsiblePeers(ctx, d.spaceId)
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

func (d *diffService) syncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	cl := spacesyncproto.NewDRPCSpaceClient(p)
	rdiff := remotediff.NewRemoteDiff(d.spaceId, cl)
	newIds, changedIds, removedIds, err := d.diff.Diff(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil && err != spacesyncproto.ErrSpaceMissing {
		return err
	}
	if err == spacesyncproto.ErrSpaceMissing {
		return d.sendPushSpaceRequest(ctx, cl)
	}

	d.pingTreesInCache(ctx, newIds)
	d.pingTreesInCache(ctx, changedIds)

	d.log.Info("sync done:", zap.Int("newIds", len(newIds)),
		zap.Int("changedIds", len(changedIds)),
		zap.Int("removedIds", len(removedIds)))
	return
}

func (d *diffService) pingTreesInCache(ctx context.Context, trees []string) {
	for _, tId := range trees {
		_, _ = d.cache.GetTree(ctx, d.spaceId, tId)
	}
}

func (d *diffService) fillDiff(objectIds []string) {
	var els = make([]ldiff.Element, 0, len(objectIds))
	for _, id := range objectIds {
		st, err := d.storage.TreeStorage(id)
		if err != nil {
			continue
		}
		heads, err := st.Heads()
		if err != nil {
			continue
		}
		els = append(els, ldiff.Element{
			Id:   id,
			Head: concatStrings(heads),
		})
	}
	d.diff.Set(els...)
}

func (d *diffService) sendPushSpaceRequest(ctx context.Context, cl spacesyncproto.DRPCSpaceClient) (err error) {
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

	_, err = cl.PushSpace(ctx, &spacesyncproto.PushSpaceRequest{
		SpaceId:     d.spaceId,
		SpaceHeader: header,
		AclRoot:     root,
	})
	return
}

func concatStrings(strs []string) string {
	var (
		b        strings.Builder
		totalLen int
	)
	for _, s := range strs {
		totalLen += len(s)
	}

	b.Grow(totalLen)
	for _, s := range strs {
		b.WriteString(s)
	}
	return b.String()
}
