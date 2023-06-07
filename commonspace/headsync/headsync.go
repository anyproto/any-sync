//go:generate mockgen -destination mock_headsync/mock_headsync.go github.com/anyproto/any-sync/commonspace/headsync DiffSyncer
package headsync

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	config2 "github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/periodicsync"
	"github.com/anyproto/any-sync/util/slice"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"sync/atomic"
	"time"
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
	DebugAllHeads() (res []TreeHeads)
	AllIds() []string
	UpdateHeads(id string, heads []string)
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	RemoveObjects(ids []string)
}

type headSync struct {
	spaceId        string
	spaceIsDeleted *atomic.Bool
	syncPeriod     int

	periodicSync       periodicsync.PeriodicSync
	storage            spacestorage.SpaceStorage
	diff               ldiff.Diff
	log                logger.CtxLogger
	syncer             DiffSyncer
	configuration      nodeconf.NodeConf
	peerManager        peermanager.PeerManager
	treeManager        treemanager.TreeManager
	credentialProvider credentialprovider.CredentialProvider
	syncStatus         syncstatus.StatusService
	deletionState      deletionstate.ObjectDeletionState
}

func New() HeadSync {
	return &headSync{}
}

var createDiffSyncer = newDiffSyncer

func (h *headSync) Init(a *app.App) (err error) {
	shared := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	cfg := a.MustComponent("config").(config2.ConfigGetter)
	h.spaceId = shared.SpaceId
	h.spaceIsDeleted = shared.SpaceIsDeleted
	h.syncPeriod = cfg.GetSpace().SyncPeriod
	h.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	h.log = log.With(zap.String("spaceId", h.spaceId))
	h.storage = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	h.diff = ldiff.New(16, 16)
	h.peerManager = a.MustComponent(peermanager.CName).(peermanager.PeerManager)
	h.credentialProvider = a.MustComponent(credentialprovider.CName).(credentialprovider.CredentialProvider)
	h.syncStatus = a.MustComponent(syncstatus.CName).(syncstatus.StatusService)
	h.treeManager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	h.deletionState = a.MustComponent(deletionstate.CName).(deletionstate.ObjectDeletionState)
	h.syncer = createDiffSyncer(h)
	sync := func(ctx context.Context) (err error) {
		// for clients cancelling the sync process
		if h.spaceIsDeleted.Load() && !h.configuration.IsResponsible(h.spaceId) {
			return spacesyncproto.ErrSpaceIsDeleted
		}
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
	initialIds, err := h.storage.StoredIds()
	if err != nil {
		return
	}
	h.fillDiff(initialIds)
	h.periodicSync.Run()
	return
}

func (h *headSync) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	if h.spaceIsDeleted.Load() {
		peerId, err := peer.CtxPeerId(ctx)
		if err != nil {
			return nil, err
		}
		// stop receiving all request for sync from clients
		if !slices.Contains(h.configuration.NodeIds(h.spaceId), peerId) {
			return nil, spacesyncproto.ErrSpaceIsDeleted
		}
	}
	return HandleRangeRequest(ctx, h.diff, req)
}

func (h *headSync) UpdateHeads(id string, heads []string) {
	h.syncer.UpdateHeads(id, heads)
}

func (h *headSync) AllIds() []string {
	return h.diff.Ids()
}

func (h *headSync) ExternalIds() []string {
	settingsId := h.storage.SpaceSettingsId()
	return slice.DiscardFromSlice(h.AllIds(), func(id string) bool {
		return id == settingsId
	})
}

func (h *headSync) DebugAllHeads() (res []TreeHeads) {
	els := h.diff.Elements()
	for _, el := range els {
		idHead := TreeHeads{
			Id:    el.Id,
			Heads: splitString(el.Head),
		}
		res = append(res, idHead)
	}
	return
}

func (h *headSync) RemoveObjects(ids []string) {
	h.syncer.RemoveObjects(ids)
}

func (h *headSync) Close(ctx context.Context) (err error) {
	h.periodicSync.Close()
	return h.syncer.Close()
}

func (h *headSync) fillDiff(objectIds []string) {
	var els = make([]ldiff.Element, 0, len(objectIds))
	for _, id := range objectIds {
		st, err := h.storage.TreeStorage(id)
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
	h.diff.Set(els...)
	if err := h.storage.WriteSpaceHash(h.diff.Hash()); err != nil {
		h.log.Error("can't write space hash", zap.Error(err))
	}
}
