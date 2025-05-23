package deletionmanager

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
)

type Deleter interface {
	Delete(ctx context.Context)
}

type deleter struct {
	st     spacestorage.SpaceStorage
	state  deletionstate.ObjectDeletionState
	getter treemanager.TreeManager
	log    logger.CtxLogger
}

func newDeleter(st spacestorage.SpaceStorage, state deletionstate.ObjectDeletionState, getter treemanager.TreeManager, log logger.CtxLogger) Deleter {
	return &deleter{st, state, getter, log}
}

func (d *deleter) Delete(ctx context.Context) {
	var (
		allQueued = d.state.GetQueued()
		spaceId   = d.st.Id()
	)
	for _, id := range allQueued {
		log := d.log.With(zap.String("treeId", id))
		shouldDelete, err := d.tryMarkDeleted(ctx, spaceId, id)
		if !shouldDelete {
			if err != nil {
				log.Error("failed to mark object as deleted", zap.Error(err))
				continue
			}
		} else {
			err = d.getter.DeleteTree(ctx, spaceId, id)
			if err != nil && !errors.Is(err, spacestorage.ErrTreeStorageAlreadyDeleted) && !errors.Is(err, synctree.ErrSyncTreeDeleted) {
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

func (d *deleter) tryMarkDeleted(ctx context.Context, spaceId, treeId string) (bool, error) {
	_, err := d.st.TreeStorage(ctx, treeId)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, treestorage.ErrUnknownTreeId) {
		return false, err
	}
	return false, d.getter.MarkTreeDeleted(context.Background(), spaceId, treeId)
}
