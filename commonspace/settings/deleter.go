package settings

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/treemanager"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"go.uber.org/zap"
)

type Deleter interface {
	Delete()
}

type deleter struct {
	st     spacestorage.SpaceStorage
	state  settingsstate.ObjectDeletionState
	getter treemanager.TreeManager
}

func newDeleter(st spacestorage.SpaceStorage, state settingsstate.ObjectDeletionState, getter treemanager.TreeManager) Deleter {
	return &deleter{st, state, getter}
}

func (d *deleter) Delete() {
	allQueued := d.state.GetQueued()
	for _, id := range allQueued {
		err := d.getter.DeleteTree(context.Background(), d.st.Id(), id)
		if err != nil && err != spacestorage.ErrTreeStorageAlreadyDeleted {
			log.With(zap.String("id", id), zap.Error(err)).Error("failed to delete object")
			continue
		}
		err = d.state.Delete(id)
		if err != nil {
			log.With(zap.String("id", id), zap.Error(err)).Error("failed to mark object as deleted")
		}
		log.With(zap.String("id", id), zap.Error(err)).Debug("object successfully deleted")
	}
}
