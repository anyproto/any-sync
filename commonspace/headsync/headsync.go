//go:generate mockgen -destination mock_headsync/mock_headsync.go github.com/anyproto/any-sync/commonspace/headsync DiffSyncer
package headsync

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/olddiff"
	"github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/object/treesyncer"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/periodicsync"
	"github.com/anyproto/any-sync/util/slice"
)

var log = logger.NewNamed(CName)

const CName = "common.commonspace.headsync"

type TreeHeads struct {
	Id    string
	Heads []string
}

type HeadSync interface {
	app.ComponentRunnable
	ExternalIds() []string
	AllIds() []string
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
}

type headSync struct {
	spaceId    string
	syncPeriod int
	settingsId string

	periodicSync       periodicsync.PeriodicSync
	storage            spacestorage.SpaceStorage
	diffContainer      ldiff.DiffContainer
	diffManager        *DiffManager
	log                logger.CtxLogger
	syncer             DiffSyncer
	configuration      nodeconf.NodeConf
	peerManager        peermanager.PeerManager
	treeSyncer         treesyncer.TreeSyncer
	credentialProvider credentialprovider.CredentialProvider
	deletionState      deletionstate.ObjectDeletionState
	syncAcl            syncacl.SyncAcl
	keyValue           kvinterfaces.KeyValueService
}

func New() HeadSync {
	return &headSync{}
}

var createDiffSyncer = newDiffSyncer

func (h *headSync) Init(a *app.App) (err error) {
	shared := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	cfg := a.MustComponent("config").(config.ConfigGetter)
	h.syncAcl = a.MustComponent(syncacl.CName).(syncacl.SyncAcl)
	h.spaceId = shared.SpaceId
	h.syncPeriod = cfg.GetSpace().SyncPeriod
	h.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	h.log = log.With(zap.String("spaceId", h.spaceId))
	h.storage = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	h.diffContainer = ldiff.NewDiffContainer(ldiff.New(32, 256), olddiff.New(32, 256))
	h.peerManager = a.MustComponent(peermanager.CName).(peermanager.PeerManager)
	h.credentialProvider = a.MustComponent(credentialprovider.CName).(credentialprovider.CredentialProvider)
	h.treeSyncer = a.MustComponent(treesyncer.CName).(treesyncer.TreeSyncer)
	h.deletionState = a.MustComponent(deletionstate.CName).(deletionstate.ObjectDeletionState)
	h.keyValue = a.MustComponent(kvinterfaces.CName).(kvinterfaces.KeyValueService)
	h.diffManager = NewDiffManager(h.diffContainer, h.storage, h.syncAcl, h.log, context.Background(), h.deletionState, h.keyValue)
	h.syncer = createDiffSyncer(h)
	sync := func(ctx context.Context) (err error) {
		return h.syncer.Sync(ctx)
	}
	h.periodicSync = periodicsync.NewPeriodicSync(h.syncPeriod, time.Minute, sync, h.log)
	// TODO: move to run?
	h.syncer.Init()
	return nil
}

func (h *headSync) Name() (name string) {
	return CName
}

func (h *headSync) Run(ctx context.Context) (err error) {
	if err := h.diffManager.FillDiff(ctx); err != nil {
		return err
	}
	h.periodicSync.Run()
	return
}

func (h *headSync) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	if req.DiffType == spacesyncproto.DiffType_V2 {
		return HandleRangeRequest(ctx, h.diffContainer.NewDiff(), req)
	} else {
		return HandleRangeRequest(ctx, h.diffContainer.OldDiff(), req)
	}
}

func (h *headSync) AllIds() []string {
	return h.diffContainer.NewDiff().Ids()
}

func (h *headSync) ExternalIds() []string {
	settingsId := h.storage.StateStorage().SettingsId()
	aclId := h.syncAcl.Id()
	keyValueId := h.keyValue.DefaultStore().Id()
	return slice.DiscardFromSlice(h.AllIds(), func(id string) bool {
		return id == settingsId || id == aclId || id == keyValueId
	})
}

func (h *headSync) Close(ctx context.Context) (err error) {
	h.syncer.Close()
	h.periodicSync.Close()
	return
}

