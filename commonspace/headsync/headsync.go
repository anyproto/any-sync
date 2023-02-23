//go:generate mockgen -destination mock_headsync/mock_headsync.go github.com/anytypeio/any-sync/commonspace/headsync DiffSyncer
package headsync

import (
	"context"
	"github.com/anytypeio/any-sync/app/ldiff"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/periodicsync"
	"go.uber.org/zap"
	"strings"
	"sync/atomic"
	"time"
)

type TreeHeads struct {
	Id    string
	Heads []string
}

type HeadSync interface {
	Init(objectIds []string, deletionState settingsstate.ObjectDeletionState)

	UpdateHeads(id string, heads []string)
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	RemoveObjects(ids []string)
	AllIds() []string
	DebugAllHeads() (res []TreeHeads)

	Close() (err error)
}

type headSync struct {
	spaceId        string
	periodicSync   periodicsync.PeriodicSync
	storage        spacestorage.SpaceStorage
	diff           ldiff.Diff
	log            logger.CtxLogger
	syncer         DiffSyncer
	spaceIsDeleted *atomic.Bool

	syncPeriod int
}

func NewHeadSync(
	spaceId string,
	spaceIsDeleted *atomic.Bool,
	syncPeriod int,
	configuration nodeconf.Configuration,
	storage spacestorage.SpaceStorage,
	peerManager peermanager.PeerManager,
	cache treegetter.TreeGetter,
	syncStatus syncstatus.StatusUpdater,
	log logger.CtxLogger) HeadSync {

	diff := ldiff.New(16, 16)
	l := log.With(zap.String("spaceId", spaceId))
	factory := spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceSyncClient)
	syncer := newDiffSyncer(spaceId, diff, peerManager, cache, storage, factory, syncStatus, l)
	sync := func(ctx context.Context) (err error) {
		if spaceIsDeleted.Load() && !configuration.IsResponsible(spaceId) {
			return spacesyncproto.ErrSpaceIsDeleted
		}
		return syncer.Sync(ctx)
	}
	periodicSync := periodicsync.NewPeriodicSync(syncPeriod, time.Minute, sync, l)

	return &headSync{
		spaceId:        spaceId,
		storage:        storage,
		syncer:         syncer,
		periodicSync:   periodicSync,
		diff:           diff,
		log:            log,
		syncPeriod:     syncPeriod,
		spaceIsDeleted: spaceIsDeleted,
	}
}

func (d *headSync) Init(objectIds []string, deletionState settingsstate.ObjectDeletionState) {
	d.fillDiff(objectIds)
	d.syncer.Init(deletionState)
	d.periodicSync.Run()
}

func (d *headSync) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	return HandleRangeRequest(ctx, d.diff, req)
}

func (d *headSync) UpdateHeads(id string, heads []string) {
	d.syncer.UpdateHeads(id, heads)
}

func (d *headSync) AllIds() []string {
	return d.diff.Ids()
}

func (d *headSync) DebugAllHeads() (res []TreeHeads) {
	els := d.diff.Elements()
	for _, el := range els {
		idHead := TreeHeads{
			Id:    el.Id,
			Heads: splitString(el.Head),
		}
		res = append(res, idHead)
	}
	return
}

func (d *headSync) RemoveObjects(ids []string) {
	d.syncer.RemoveObjects(ids)
}

func (d *headSync) Close() (err error) {
	d.periodicSync.Close()
	return nil
}

func (d *headSync) fillDiff(objectIds []string) {
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
	if err := d.storage.WriteSpaceHash(d.diff.Hash()); err != nil {
		d.log.Error("can't write space hash", zap.Error(err))
	}
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

func splitString(str string) (res []string) {
	const cidLen = 59
	for i := 0; i < len(str); i += cidLen {
		res = append(res, str[i:i+cidLen])
	}
	return
}
