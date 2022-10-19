//go:generate mockgen -destination mock_diffservice/mock_diffservice.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice DiffSyncer,PeriodicSync
package diffservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff"
	"go.uber.org/zap"
	"strings"
)

type DiffService interface {
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	UpdateHeads(id string, heads []string)
	RemoveObject(id string)
	AllIds() []string

	Init(objectIds []string)
	Close() (err error)
}

type diffService struct {
	spaceId      string
	periodicSync PeriodicSync
	storage      storage.SpaceStorage
	diff         ldiff.Diff
	log          *zap.Logger

	syncPeriod int
}

func NewDiffService(
	spaceId string,
	syncPeriod int,
	storage storage.SpaceStorage,
	confConnector nodeconf.ConfConnector,
	cache treegetter.TreeGetter,
	log *zap.Logger) DiffService {

	diff := ldiff.New(16, 16)
	l := log.With(zap.String("spaceId", spaceId))
	factory := spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceClient)
	syncer := newDiffSyncer(spaceId, diff, confConnector, cache, storage, factory, l)
	periodicSync := newPeriodicSync(syncPeriod, syncer, l)

	return &diffService{
		spaceId:      spaceId,
		storage:      storage,
		periodicSync: periodicSync,
		diff:         diff,
		log:          log,
		syncPeriod:   syncPeriod,
	}
}

func (d *diffService) Init(objectIds []string) {
	d.fillDiff(objectIds)
	d.periodicSync.Run()
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

func (d *diffService) AllIds() []string {
	return d.diff.Ids()
}

func (d *diffService) RemoveObject(id string) {
	// TODO: add space document to remove ids
	d.diff.RemoveId(id)
}

func (d *diffService) Close() (err error) {
	d.periodicSync.Close()
	return nil
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
