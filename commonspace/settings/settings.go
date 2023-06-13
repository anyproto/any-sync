package settings

import (
	"context"
	"sync/atomic"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objecttreebuilder"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
)

const CName = "common.commonspace.settings"

type Settings interface {
	DeleteTree(ctx context.Context, id string) (err error)
	SpaceDeleteRawChange(ctx context.Context) (raw *treechangeproto.RawTreeChangeWithId, err error)
	DeleteSpace(ctx context.Context, deleteChange *treechangeproto.RawTreeChangeWithId) (err error)
	SettingsObject() SettingsObject
	app.ComponentRunnable
}

func New() Settings {
	return &settings{}
}

type settings struct {
	account        accountservice.Service
	treeManager    treemanager.TreeManager
	storage        spacestorage.SpaceStorage
	configuration  nodeconf.NodeConf
	deletionState  deletionstate.ObjectDeletionState
	headsync       headsync.HeadSync
	treeBuilder    objecttreebuilder.TreeBuilderComponent
	spaceIsDeleted *atomic.Bool

	settingsObject SettingsObject
}

func (s *settings) Init(a *app.App) (err error) {
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.treeManager = app.MustComponent[treemanager.TreeManager](a)
	s.headsync = a.MustComponent(headsync.CName).(headsync.HeadSync)
	s.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	s.deletionState = a.MustComponent(deletionstate.CName).(deletionstate.ObjectDeletionState)
	s.treeBuilder = a.MustComponent(objecttreebuilder.CName).(objecttreebuilder.TreeBuilderComponent)

	sharedState := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	s.storage = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	s.spaceIsDeleted = sharedState.SpaceIsDeleted

	deps := Deps{
		BuildFunc: func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error) {
			res, err := s.treeBuilder.BuildTree(ctx, id, objecttreebuilder.BuildTreeOpts{
				Listener: listener,
				// space settings document should not have empty data
				TreeBuilder: objecttree.BuildObjectTree,
			})
			log.Debug("building settings tree", zap.String("id", id), zap.String("spaceId", sharedState.SpaceId))
			if err != nil {
				return
			}
			t = res.(synctree.SyncTree)
			return
		},
		Account:       s.account,
		TreeManager:   s.treeManager,
		Store:         s.storage,
		Configuration: s.configuration,
		DeletionState: s.deletionState,
		Provider:      s.headsync,
		OnSpaceDelete: s.onSpaceDelete,
	}
	s.settingsObject = NewSettingsObject(deps, sharedState.SpaceId)
	return nil
}

func (s *settings) Name() (name string) {
	return CName
}

func (s *settings) Run(ctx context.Context) (err error) {
	return s.settingsObject.Init(ctx)
}

func (s *settings) Close(ctx context.Context) (err error) {
	return s.settingsObject.Close()
}

func (s *settings) DeleteTree(ctx context.Context, id string) (err error) {
	return s.settingsObject.DeleteObject(id)
}

func (s *settings) SpaceDeleteRawChange(ctx context.Context) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return s.settingsObject.SpaceDeleteRawChange()
}

func (s *settings) DeleteSpace(ctx context.Context, deleteChange *treechangeproto.RawTreeChangeWithId) (err error) {
	return s.settingsObject.DeleteSpace(ctx, deleteChange)
}

func (s *settings) onSpaceDelete() {
	err := s.storage.SetSpaceDeleted()
	if err != nil {
		log.Warn("failed to set space deleted")
	}
	s.spaceIsDeleted.Swap(true)
}

func (s *settings) SettingsObject() SettingsObject {
	return s.settingsObject
}
