package settings

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
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
	deletionState deletionstate.ObjectDeletionState,
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
	deletionState deletionstate.ObjectDeletionState
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
	// we should delete space
	log.Debug("deleting space")
	if d.isResponsible {
		mapIds := map[string]struct{}{}
		for _, id := range d.provider.AllIds() {
			if id != d.settingsId {
				mapIds[id] = struct{}{}
			}
		}
		d.deletionState.Add(mapIds)
	}
	d.onSpaceDelete()
	return nil
}
