package settings

import (
	"context"
	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objecttreebuilder"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/nodeconf"
	"go.uber.org/zap"
)

const CName = "common.commonspace.settings"

type Settings interface {
	app.ComponentRunnable
}

type settings struct {
	account       accountservice.Service
	treeManager   treemanager.TreeManager
	storage       spacestorage.SpaceStorage
	configuration nodeconf.NodeConf
	deletionState deletionstate.ObjectDeletionState
	headsync      headsync.HeadSync
	spaceActions  spacestate.SpaceActions
	treeBuilder   objecttreebuilder.TreeBuilder

	settingsObject SettingsObject
}

func (s *settings) Run(ctx context.Context) (err error) {
	return s.settingsObject.Init(ctx)
}

func (s *settings) Close(ctx context.Context) (err error) {
	return s.settingsObject.Close()
}

func (s *settings) Init(a *app.App) (err error) {
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.treeManager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	s.headsync = a.MustComponent(headsync.CName).(headsync.HeadSync)
	s.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	s.deletionState = a.MustComponent(deletionstate.CName).(deletionstate.ObjectDeletionState)
	s.treeBuilder = a.MustComponent(objecttreebuilder.CName).(objecttreebuilder.TreeBuilder)

	sharedState := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	s.spaceActions = sharedState.Actions
	s.storage = sharedState.SpaceStorage

	deps := Deps{
		BuildFunc: func(ctx context.Context, id string, listener updatelistener.UpdateListener) (t synctree.SyncTree, err error) {
			res, err := s.treeBuilder.BuildTree(ctx, id, objecttreebuilder.BuildTreeOpts{
				Listener:           listener,
				WaitTreeRemoteSync: false,
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
		OnSpaceDelete: s.spaceActions.OnSpaceDelete,
	}
	s.settingsObject = NewSettingsObject(deps, sharedState.SpaceId)
	return nil
}

func (s *settings) Name() (name string) {
	return CName
}
