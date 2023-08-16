//go:generate mockgen -destination mock_deletionmanager/mock_deletionmanager.go github.com/anyproto/any-sync/commonspace/deletionmanager DeletionManager,Deleter,SpaceIdsProvider
package deletionmanager

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"go.uber.org/zap"
)

type SpaceIdsProvider interface {
	app.Component
	AllIds() []string
}

type DeletionManager interface {
	app.ComponentRunnable
	UpdateState(ctx context.Context, state *settingsstate.State) (err error)
}

func New() DeletionManager {
	return &deletionManager{}
}

const CName = "common.commonspace.deletionmanager"

var log = logger.NewNamed(CName)

type deletionManager struct {
	deletionState deletionstate.ObjectDeletionState
	deleter       Deleter
	loop          *deleteLoop
	log           logger.CtxLogger

	spaceId string
}

func (d *deletionManager) Init(a *app.App) (err error) {
	state := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	storage := a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	d.log = log.With(zap.String("spaceId", state.SpaceId), zap.String("settingsId", storage.SpaceSettingsId()))
	d.deletionState = a.MustComponent(deletionstate.CName).(deletionstate.ObjectDeletionState)
	treeManager := a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	d.deleter = newDeleter(storage, d.deletionState, treeManager, d.log)
	d.loop = newDeleteLoop(d.deleter.Delete)
	d.deletionState.AddObserver(func(ids []string) {
		d.loop.notify()
	})
	return
}

func (d *deletionManager) Name() (name string) {
	return CName
}

func (d *deletionManager) Run(ctx context.Context) (err error) {
	d.loop.Run()
	return
}

func (d *deletionManager) Close(ctx context.Context) (err error) {
	d.loop.Close()
	return
}

func (d *deletionManager) UpdateState(ctx context.Context, state *settingsstate.State) error {
	d.deletionState.Add(state.DeletedIds)
	return nil
}
