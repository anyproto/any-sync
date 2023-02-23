package settings

import (
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/util/slice"
	"time"
)

type SpaceIdsProvider interface {
	AllIds() []string
	RemoveObjects(ids []string)
}

type SpaceDeleter interface {
	DeleteSpace(spaceId string)
}

type DeletionManager interface {
	UpdateState(state *settingsstate.State) (err error)
}

func newDeletionManager(
	spaceId string,
	settingsId string,
	isResponsible bool,
	deletionInterval time.Duration,
	deletionState settingsstate.ObjectDeletionState,
	provider SpaceIdsProvider,
	onSpaceDelete func()) DeletionManager {
	return &deletionManager{
		isResponsible:    isResponsible,
		spaceId:          spaceId,
		settingsId:       settingsId,
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
	settingsId       string
	isResponsible    bool
	onSpaceDelete    func()
}

func (d *deletionManager) UpdateState(state *settingsstate.State) (err error) {
	err = d.deletionState.Add(state.DeletedIds)
	if err != nil {
		log.Warn("failed to add deleted ids to deletion state")
	}

	if state.DeleterId != "" {
		spaceDeleter, ok := d.treeGetter.(SpaceDeleter)
		if ok {
			spaceDeleter.DeleteSpace(d.spaceId)
		}
		if d.isResponsible {
			allIds := slice.DiscardFromSlice(d.provider.AllIds(), func(id string) bool {
				return id == d.settingsId
			})
			err = d.deletionState.Add(allIds)
			if err != nil {
				return
			}
			d.provider.RemoveObjects(allIds)
		}
		d.onSpaceDelete()
	}
	return
}
