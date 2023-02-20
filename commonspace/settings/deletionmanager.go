package settings

import (
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/deletionstate"
	"time"
)

type SpaceIdsProvider interface {
	AllIds() []string
}

type SpaceDeleter interface {
	DeleteSpace(spaceId string)
}

type DeletionManager interface {
	UpdateState(state *State) (err error)
}

func newDeletionManager(
	spaceId string,
	deletionState deletionstate.DeletionState,
	provider SpaceIdsProvider,
	deletionInterval time.Duration) DeletionManager {
	return &deletionManager{
		spaceId:          spaceId,
		deletionState:    deletionState,
		provider:         provider,
		deletionInterval: deletionInterval,
	}
}

type deletionManager struct {
	deletionState    deletionstate.DeletionState
	provider         SpaceIdsProvider
	treeGetter       treegetter.TreeGetter
	deletionInterval time.Duration
	spaceId          string
}

func (d *deletionManager) UpdateState(state *State) (err error) {
	err = d.deletionState.Add(state.DeletedIds)
	if err != nil {
		log.Warn("failed to add deleted ids to deletion state")
	}
	if !state.SpaceDeletionDate.IsZero() && state.SpaceDeletionDate.Add(d.deletionInterval).Before(time.Now()) {
		err = d.deletionState.Add(d.provider.AllIds())
		spaceDeleter, ok := d.treeGetter.(SpaceDeleter)
		if ok {
			spaceDeleter.DeleteSpace(d.spaceId)
		}
	}
	return
}
