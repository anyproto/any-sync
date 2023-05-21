package settings

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/treemanager"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/util/slice"
	"go.uber.org/zap"
)

type SpaceIdsProvider interface {
	AllIds() []string
}

type DeletionManager interface {
	UpdateState(ctx context.Context, state *settingsstate.State) (err error)
}

func newDeletionManager(
	spaceId string,
	settingsId string,
	isResponsible bool,
	treeManager treemanager.TreeManager,
	deletionState settingsstate.ObjectDeletionState,
	provider SpaceIdsProvider,
	onSpaceDelete func()) DeletionManager {
	return &deletionManager{
		treeManager:   treeManager,
		isResponsible: isResponsible,
		spaceId:       spaceId,
		settingsId:    settingsId,
		deletionState: deletionState,
		provider:      provider,
		onSpaceDelete: onSpaceDelete,
	}
}

type deletionManager struct {
	deletionState settingsstate.ObjectDeletionState
	provider      SpaceIdsProvider
	treeManager   treemanager.TreeManager
	spaceId       string
	settingsId    string
	isResponsible bool
	onSpaceDelete func()
}

func (d *deletionManager) UpdateState(ctx context.Context, state *settingsstate.State) error {
	log := log.With(zap.String("spaceId", d.spaceId))
	d.deletionState.Add(state.DeletedIds)
	if state.DeleterId == "" {
		return nil
	}
	log.Debug("deleting space")
	if d.isResponsible {
		allIds := slice.DiscardFromSlice(d.provider.AllIds(), func(id string) bool {
			return id == d.settingsId
		})
		d.deletionState.Add(allIds)
	}
	d.onSpaceDelete()
	return nil
}
