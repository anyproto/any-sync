package deletionmanager

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
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
		// Delete bound children alongside parent
		d.deleteBoundChildren(ctx, spaceId, id)
	}
}

func (d *deleter) deleteBoundChildren(ctx context.Context, spaceId, parentId string) {
	children, err := d.st.HeadStorage().GetEntriesByParentId(ctx, parentId)
	if err != nil {
		d.log.Error("failed to get bound children", zap.String("parentId", parentId), zap.Error(err))
		return
	}
	for _, child := range children {
		if child.DeletedStatus >= headstorage.DeletedStatusDeleted {
			continue
		}
		childLog := d.log.With(zap.String("treeId", child.Id), zap.String("parentId", parentId))
		shouldDelete, err := d.tryMarkDeleted(ctx, spaceId, child.Id)
		if !shouldDelete {
			if err != nil {
				childLog.Error("failed to mark bound child as deleted", zap.Error(err))
				continue
			}
		} else {
			err = d.getter.DeleteTree(ctx, spaceId, child.Id)
			if err != nil && !errors.Is(err, spacestorage.ErrTreeStorageAlreadyDeleted) && !errors.Is(err, synctree.ErrSyncTreeDeleted) {
				childLog.Error("failed to delete bound child", zap.Error(err))
				continue
			}
		}
		err = d.state.Delete(child.Id)
		if err != nil {
			childLog.Error("failed to mark bound child as deleted in state", zap.Error(err))
		}
		childLog.Debug("bound child successfully deleted")
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
