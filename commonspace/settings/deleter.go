package settings

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"go.uber.org/zap"
)

type Deleter interface {
	Delete()
}

type deleter struct {
	st     spacestorage.SpaceStorage
	state  deletionstate.ObjectDeletionState
	getter treemanager.TreeManager
}

func newDeleter(st spacestorage.SpaceStorage, state deletionstate.ObjectDeletionState, getter treemanager.TreeManager) Deleter {
	return &deleter{st, state, getter}
}

func (d *deleter) Delete() {
	var (
		allQueued = d.state.GetQueued()
		spaceId   = d.st.Id()
	)
	for _, id := range allQueued {
		log := log.With(zap.String("treeId", id), zap.String("spaceId", spaceId))
		shouldDelete, err := d.tryMarkDeleted(spaceId, id)
		if !shouldDelete {
			if err != nil {
				log.Error("failed to mark object as deleted", zap.Error(err))
				continue
			}
		} else {
			err = d.getter.DeleteTree(context.Background(), spaceId, id)
			if err != nil && err != spacestorage.ErrTreeStorageAlreadyDeleted {
				log.Error("failed to delete object", zap.Error(err))
				continue
			}
		}
		err = d.state.Delete(id)
		if err != nil {
			log.Error("failed to mark object as deleted", zap.Error(err))
		}
		log.Debug("object successfully deleted", zap.Error(err))
	}
}

func (d *deleter) tryMarkDeleted(spaceId, treeId string) (bool, error) {
	_, err := d.st.TreeStorage(treeId)
	if err == nil {
		return true, nil
	}
	if err != treestorage.ErrUnknownTreeId {
		return false, err
	}
	return false, d.getter.MarkTreeDeleted(context.Background(), spaceId, treeId)
}
