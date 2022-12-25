//go:generate mockgen -destination mock_headsync/mock_headsync.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/headsync DiffSyncer
package headsync

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings/deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncstatus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/periodicsync"
	"go.uber.org/zap"
	"strings"
	"time"
)

type TreeHeads struct {
	Id    string
	Heads []string
}

type HeadSync interface {
	UpdateHeads(id string, heads []string)
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	RemoveObjects(ids []string)
	AllIds() []string
	DebugAllHeads() (res []TreeHeads)

	Init(objectIds []string, deletionState deletionstate.DeletionState)
	Close() (err error)
}

type headSync struct {
	spaceId      string
	periodicSync periodicsync.PeriodicSync
	storage      storage.SpaceStorage
	diff         ldiff.Diff
	log          *zap.Logger
	syncer       DiffSyncer

	syncPeriod int
}

func NewHeadSync(
	spaceId string,
	syncPeriod int,
	storage storage.SpaceStorage,
	confConnector nodeconf.ConfConnector,
	cache treegetter.TreeGetter,
	syncStatus syncstatus.SyncStatusUpdater,
	log *zap.Logger) HeadSync {

	diff := ldiff.New(16, 16)
	l := log.With(zap.String("spaceId", spaceId))
	factory := spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceClient)
	syncer := newDiffSyncer(spaceId, diff, confConnector, cache, storage, factory, syncStatus, l)
	periodicSync := periodicsync.NewPeriodicSync(syncPeriod, time.Minute, syncer.Sync, l)

	return &headSync{
		spaceId:      spaceId,
		storage:      storage,
		syncer:       syncer,
		periodicSync: periodicSync,
		diff:         diff,
		log:          log,
		syncPeriod:   syncPeriod,
	}
}

func (d *headSync) Init(objectIds []string, deletionState deletionstate.DeletionState) {
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
