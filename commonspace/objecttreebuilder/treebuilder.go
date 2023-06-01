package objecttreebuilder

import (
	"context"
	"errors"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/headsync"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/objectsync/syncclient"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
	"sync/atomic"
)

type BuildTreeOpts struct {
	Listener           updatelistener.UpdateListener
	WaitTreeRemoteSync bool
	TreeBuilder        objecttree.BuildObjectTreeFunc
}

const CName = "common.commonspace.objecttreebuilder"

var log = logger.NewNamed(CName)
var ErrSpaceClosed = errors.New("space is closed")

type HistoryTreeOpts struct {
	BeforeId      string
	Include       bool
	BuildFullTree bool
}

type TreeBuilder interface {
	BuildTree(ctx context.Context, id string, opts BuildTreeOpts) (t objecttree.ObjectTree, err error)
	BuildHistoryTree(ctx context.Context, id string, opts HistoryTreeOpts) (t objecttree.HistoryTree, err error)
	CreateTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error)
	PutTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, listener updatelistener.UpdateListener) (t objecttree.ObjectTree, err error)
}

type TreeBuilderComponent interface {
	app.Component
	TreeBuilder
}

func New() TreeBuilderComponent {
	return &treeBuilder{}
}

type treeBuilder struct {
	syncClient      syncclient.SyncClient
	configuration   nodeconf.NodeConf
	headsNotifiable synctree.HeadNotifiable
	peerManager     peermanager.PeerManager
	spaceStorage    spacestorage.SpaceStorage
	syncStatus      syncstatus.StatusUpdater
	objectSync      objectsync.ObjectSync

	log       logger.CtxLogger
	builder   objecttree.BuildObjectTreeFunc
	spaceId   string
	aclList   list.AclList
	treesUsed *atomic.Int32
	isClosed  *atomic.Bool
}

func (t *treeBuilder) Init(a *app.App) (err error) {
	state := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	t.spaceId = state.SpaceId
	t.isClosed = state.SpaceIsClosed
	t.treesUsed = state.TreesUsed
	t.builder = state.TreeBuilderFunc
	t.aclList = a.MustComponent(syncacl.CName).(*syncacl.SyncAcl)
	t.spaceStorage = a.MustComponent(spacestorage.StorageName).(spacestorage.SpaceStorage)
	t.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	t.headsNotifiable = a.MustComponent(headsync.CName).(headsync.HeadSync)
	t.syncStatus = a.MustComponent(syncstatus.CName).(syncstatus.StatusUpdater)
	t.peerManager = a.MustComponent(peermanager.ManagerName).(peermanager.PeerManager)
	t.objectSync = a.MustComponent(objectsync.CName).(objectsync.ObjectSync)
	t.log = log.With(zap.String("spaceId", t.spaceId))
	return nil
}

func (t *treeBuilder) Name() (name string) {
	return CName
}

func (t *treeBuilder) BuildTree(ctx context.Context, id string, opts BuildTreeOpts) (ot objecttree.ObjectTree, err error) {
	if t.isClosed.Load() {
		// TODO: change to real error
		err = ErrSpaceClosed
		return
	}
	treeBuilder := opts.TreeBuilder
	if treeBuilder == nil {
		treeBuilder = t.builder
	}
	deps := synctree.BuildDeps{
		SpaceId:            t.spaceId,
		SyncClient:         t.syncClient,
		Configuration:      t.configuration,
		HeadNotifiable:     t.headsNotifiable,
		Listener:           opts.Listener,
		AclList:            t.aclList,
		SpaceStorage:       t.spaceStorage,
		OnClose:            t.onClose,
		SyncStatus:         t.syncStatus,
		WaitTreeRemoteSync: opts.WaitTreeRemoteSync,
		PeerGetter:         t.peerManager,
		BuildObjectTree:    treeBuilder,
	}
	t.treesUsed.Add(1)
	t.log.Debug("incrementing counter", zap.String("id", id), zap.Int32("trees", t.treesUsed.Load()))
	if ot, err = synctree.BuildSyncTreeOrGetRemote(ctx, id, deps); err != nil {
		t.treesUsed.Add(-1)
		t.log.Debug("decrementing counter, load failed", zap.String("id", id), zap.Int32("trees", t.treesUsed.Load()), zap.Error(err))
		return nil, err
	}
	return
}

func (t *treeBuilder) BuildHistoryTree(ctx context.Context, id string, opts HistoryTreeOpts) (ot objecttree.HistoryTree, err error) {
	if t.isClosed.Load() {
		// TODO: change to real error
		err = ErrSpaceClosed
		return
	}

	params := objecttree.HistoryTreeParams{
		AclList:         t.aclList,
		BeforeId:        opts.BeforeId,
		IncludeBeforeId: opts.Include,
		BuildFullTree:   opts.BuildFullTree,
	}
	params.TreeStorage, err = t.spaceStorage.TreeStorage(id)
	if err != nil {
		return
	}
	return objecttree.BuildHistoryTree(params)
}

func (t *treeBuilder) CreateTree(ctx context.Context, payload objecttree.ObjectTreeCreatePayload) (res treestorage.TreeStorageCreatePayload, err error) {
	if t.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	root, err := objecttree.CreateObjectTreeRoot(payload, t.aclList)
	if err != nil {
		return
	}

	res = treestorage.TreeStorageCreatePayload{
		RootRawChange: root,
		Changes:       []*treechangeproto.RawTreeChangeWithId{root},
		Heads:         []string{root.Id},
	}
	return
}

func (t *treeBuilder) PutTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload, listener updatelistener.UpdateListener) (ot objecttree.ObjectTree, err error) {
	if t.isClosed.Load() {
		err = ErrSpaceClosed
		return
	}
	deps := synctree.BuildDeps{
		SpaceId:         t.spaceId,
		SyncClient:      t.syncClient,
		Configuration:   t.configuration,
		HeadNotifiable:  t.headsNotifiable,
		Listener:        listener,
		AclList:         t.aclList,
		SpaceStorage:    t.spaceStorage,
		OnClose:         t.onClose,
		SyncStatus:      t.syncStatus,
		PeerGetter:      t.peerManager,
		BuildObjectTree: t.builder,
	}
	ot, err = synctree.PutSyncTree(ctx, payload, deps)
	if err != nil {
		return
	}
	t.treesUsed.Add(1)
	t.log.Debug("incrementing counter", zap.String("id", payload.RootRawChange.Id), zap.Int32("trees", t.treesUsed.Load()))
	return
}

func (t *treeBuilder) onClose(id string) {
	t.treesUsed.Add(-1)
	log.Debug("decrementing counter", zap.String("id", id), zap.Int32("trees", t.treesUsed.Load()), zap.String("spaceId", t.spaceId))
	_ = t.objectSync.CloseThread(id)
}
