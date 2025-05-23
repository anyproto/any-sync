package settings

import (
	"context"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/deletionmanager"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/objecttreebuilder"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.commonspace.settings"

type Settings interface {
	DeleteTree(ctx context.Context, id string) (err error)
	SettingsObject() SettingsObject
	app.ComponentRunnable
}

func New() Settings {
	return &settings{}
}

type settings struct {
	account       accountservice.Service
	storage       spacestorage.SpaceStorage
	configuration nodeconf.NodeConf
	treeBuilder   objecttreebuilder.TreeBuilderComponent

	settingsObject SettingsObject
}

func (s *settings) Init(a *app.App) (err error) {
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	s.treeBuilder = a.MustComponent(objecttreebuilder.CName).(objecttreebuilder.TreeBuilderComponent)
	sharedState := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	s.storage = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)

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
		Store:         s.storage,
		Configuration: s.configuration,
		DelManager:    a.MustComponent(deletionmanager.CName).(deletionmanager.DeletionManager),
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
	return s.settingsObject.DeleteObject(ctx, id)
}

func (s *settings) SettingsObject() SettingsObject {
	return s.settingsObject
}
