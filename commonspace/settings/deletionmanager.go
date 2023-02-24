package settings

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/util/slice"
	"go.uber.org/zap"
	"time"
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
	deletionInterval time.Duration,
	treeGetter treegetter.TreeGetter,
	deletionState settingsstate.ObjectDeletionState,
	provider SpaceIdsProvider,
	onSpaceDelete func()) DeletionManager {
	return &deletionManager{
		treeGetter:       treeGetter,
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

func (d *deletionManager) UpdateState(ctx context.Context, state *settingsstate.State) error {
	log := log.With(zap.String("spaceId", d.spaceId))
	err := d.deletionState.Add(state.DeletedIds)
	if err != nil {
		log.Warn("failed to add deleted ids to deletion state")
	}
	if state.DeleterId == "" {
		return nil
	}
	log.Debug("deleting space")
	err = d.treeGetter.DeleteSpace(ctx, d.spaceId)
	if err != nil {
		log.Debug("failed to notify on space deletion", zap.Error(err))
	}
	if d.isResponsible {
		allIds := slice.DiscardFromSlice(d.provider.AllIds(), func(id string) bool {
			return id == d.settingsId
		})
		err := d.deletionState.Add(allIds)
		if err != nil {
			return err
		}
	}
	d.onSpaceDelete()
	return nil
}
