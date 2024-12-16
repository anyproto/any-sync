//go:generate mockgen -destination mock_headsync/mock_headsync.go github.com/anyproto/any-sync/commonspace/headsync DiffSyncer
package headsync

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
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
	DebugAllHeads() (res []TreeHeads)
	AllIds() []string
	UpdateHeads(id string, heads []string)
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	RemoveObjects(ids []string)
}

type headSync struct {
	spaceId    string
	syncPeriod int

	periodicSync       periodicsync.PeriodicSync
	storage            spacestorage.SpaceStorage
	diff               ldiff.Diff
	log                logger.CtxLogger
	syncer             DiffSyncer
	configuration      nodeconf.NodeConf
	peerManager        peermanager.PeerManager
	treeSyncer         treesyncer.TreeSyncer
	credentialProvider credentialprovider.CredentialProvider
	deletionState      deletionstate.ObjectDeletionState
	syncAcl            syncacl.SyncAcl
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
	h.diff = ldiff.New(32, 256)
	h.peerManager = a.MustComponent(peermanager.CName).(peermanager.PeerManager)
	h.credentialProvider = a.MustComponent(credentialprovider.CName).(credentialprovider.CredentialProvider)
	h.treeSyncer = a.MustComponent(treesyncer.CName).(treesyncer.TreeSyncer)
	h.deletionState = a.MustComponent(deletionstate.CName).(deletionstate.ObjectDeletionState)
	h.syncer = createDiffSyncer(h)
	sync := func(ctx context.Context) (err error) {
		return h.syncer.Sync(ctx)
	}
	h.periodicSync = periodicsync.NewPeriodicSync(h.syncPeriod, time.Minute, sync, h.log)
	h.syncAcl.SetHeadUpdater(h)
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
	resp, err = HandleRangeRequest(ctx, h.diff, req)
	if err != nil {
		return
	}
	// this is done to fix the problem with compatibility with old clients
	resp.DiffType = spacesyncproto.DiffType_Precalculated
	return
}

func (h *headSync) UpdateHeads(id string, heads []string) {
	h.syncer.UpdateHeads(id, heads)
}

func (h *headSync) AllIds() []string {
	return h.diff.Ids()
}

func (h *headSync) ExternalIds() []string {
	settingsId := h.storage.SpaceSettingsId()
	aclId := h.syncAcl.Id()
	return slice.DiscardFromSlice(h.AllIds(), func(id string) bool {
		return id == settingsId || id == aclId
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
	return
}

var checkDerived = (*headSync).isDerived

func (h *headSync) isDerived(storage treestorage.TreeStorage) (isDerived bool, err error) {
	r, err := storage.Root()
	if err != nil {
		return
	}
	root, err := objecttree.UnmarshallRoot(r)
	if err != nil {
		return
	}
	return root.IsDerived, nil
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
		if len(heads) == 0 {
			log.Warn("empty heads", zap.String("id", id))
			continue
		}
		isDerived, err := checkDerived(h, st)
		if err != nil {
			log.Warn("can't get derived flag", zap.Error(err))
			continue
		}
		if isDerived && heads[0] == id {
			// this is an empty derived object, we don't need to sync it
			continue
		}
		els = append(els, ldiff.Element{
			Id:   id,
			Head: concatStrings(heads),
		})
	}
	els = append(els, ldiff.Element{
		Id:   h.syncAcl.Id(),
		Head: h.syncAcl.Head().Id,
	})
	log.Debug("setting acl", zap.String("aclId", h.syncAcl.Id()), zap.String("headId", h.syncAcl.Head().Id))
	h.diff.Set(els...)
	if err := h.storage.WriteSpaceHash(h.diff.Hash()); err != nil {
		h.log.Error("can't write space hash", zap.Error(err))
	}
}
