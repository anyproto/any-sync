package settings

import (
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"time"
)

type SpaceIdsProvider interface {
	AllIds() []string
}

type SpaceDeleter interface {
	DeleteSpace(spaceId string)
}

type DeletionManager interface {
	UpdateState(state *settingsstate.State) (err error)
}

func newDeletionManager(
	spaceId string,
	deletionInterval time.Duration,
	deletionState settingsstate.ObjectDeletionState,
	provider SpaceIdsProvider,
	onSpaceDelete func()) DeletionManager {
	return &deletionManager{
		spaceId:          spaceId,
		deletionState:    deletionState,
		provider:         provider,
		deletionInterval: deletionInterval,
		onSpaceDelete:    onSpaceDelete,
	}
}

type deletionManager struct {
	deletionState    settingsstate.ObjectDeletionState
	provider         SpaceIdsProvider
	treeGetter       treegetter.TreeGetter
	deletionInterval time.Duration
	spaceId          string
	onSpaceDelete    func()
}

func (d *deletionManager) UpdateState(state *settingsstate.State) (err error) {
	err = d.deletionState.Add(state.DeletedIds)
	if err != nil {
		log.Warn("failed to add deleted ids to deletion state")
	}
	if !state.SpaceDeletionDate.IsZero() {
		spaceDeleter, ok := d.treeGetter.(SpaceDeleter)
		if ok {
			spaceDeleter.DeleteSpace(d.spaceId)
		}
		if state.SpaceDeletionDate.Add(d.deletionInterval).Before(time.Now()) {
			err = d.deletionState.Add(d.provider.AllIds())
			d.onSpaceDelete()
		}
	}
	return
}
