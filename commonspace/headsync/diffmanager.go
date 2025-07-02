package headsync

import (
	"context"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

type DiffManager struct {
	diffContainer ldiff.DiffContainer
	storage       spacestorage.SpaceStorage
	syncAcl       syncacl.SyncAcl
	log           logger.CtxLogger
	ctx           context.Context
	deletionState deletionstate.ObjectDeletionState
	keyValue      kvinterfaces.KeyValueService
}

func NewDiffManager(
	diffContainer ldiff.DiffContainer,
	storage spacestorage.SpaceStorage,
	syncAcl syncacl.SyncAcl,
	log logger.CtxLogger,
	ctx context.Context,
	deletionState deletionstate.ObjectDeletionState,
	keyValue kvinterfaces.KeyValueService,
) *DiffManager {
	return &DiffManager{
		diffContainer: diffContainer,
		storage:       storage,
		syncAcl:       syncAcl,
		log:           log,
		ctx:           ctx,
		deletionState: deletionState,
		keyValue:      keyValue,
	}
}

func (dm *DiffManager) FillDiff(ctx context.Context) error {
	var els = make([]ldiff.Element, 0, 100)
	var aclOrStorage []ldiff.Element
	err := dm.storage.HeadStorage().IterateEntries(ctx, headstorage.IterOpts{}, func(entry headstorage.HeadsEntry) (bool, error) {
		if entry.IsDerived && entry.Heads[0] == entry.Id {
			return true, nil
		}
		if entry.CommonSnapshot != "" {
			els = append(els, ldiff.Element{
				Id:   entry.Id,
				Head: concatStrings(entry.Heads),
			})
		} else {
			// this whole stuff is done to prevent storage hash from being set to old diff
			aclOrStorage = append(aclOrStorage, ldiff.Element{
				Id:   entry.Id,
				Head: concatStrings(entry.Heads),
			})
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	els = append(els, ldiff.Element{
		Id:   dm.syncAcl.Id(),
		Head: dm.syncAcl.Head().Id,
	})
	log.Debug("setting acl", zap.String("aclId", dm.syncAcl.Id()), zap.String("headId", dm.syncAcl.Head().Id))
	dm.diffContainer.Set(els...)
	// acl will be set twice to the diff but it doesn't matter
	dm.diffContainer.NewDiff().Set(aclOrStorage...)
	oldHash := dm.diffContainer.OldDiff().Hash()
	newHash := dm.diffContainer.NewDiff().Hash()
	if err := dm.storage.StateStorage().SetHash(ctx, oldHash, newHash); err != nil {
		dm.log.Error("can't write space hash", zap.Error(err))
		return err
	}
	return nil
}

func (dm *DiffManager) TryDiff(ctx context.Context, rdiff RemoteDiff) (newIds, changedIds, removedIds []string, needsSync bool, err error) {
	needsSync, diff, err := dm.diffContainer.DiffTypeCheck(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if needsSync {
		newIds, changedIds, removedIds, err = diff.Diff(ctx, rdiff)
		err = rpcerr.Unwrap(err)
		if err != nil {
			return nil, nil, nil, needsSync, err
		}
	}
	return newIds, changedIds, removedIds, needsSync, nil
}

func (dm *DiffManager) UpdateHeads(update headstorage.HeadsUpdate) {
	if update.DeletedStatus != nil {
		_ = dm.diffContainer.RemoveId(update.Id)
	} else {
		if dm.deletionState.Exists(update.Id) {
			return
		}
		if update.IsDerived != nil && *update.IsDerived && len(update.Heads) == 1 && update.Heads[0] == update.Id {
			return
		}
		if update.Id == dm.keyValue.DefaultStore().Id() {
			dm.diffContainer.NewDiff().Set(ldiff.Element{
				Id:   update.Id,
				Head: concatStrings(update.Heads),
			})
		} else {
			dm.diffContainer.Set(ldiff.Element{
				Id:   update.Id,
				Head: concatStrings(update.Heads),
			})
		}
	}
	// probably we should somehow batch the updates
	oldHash := dm.diffContainer.OldDiff().Hash()
	newHash := dm.diffContainer.NewDiff().Hash()
	err := dm.storage.StateStorage().SetHash(dm.ctx, oldHash, newHash)
	if err != nil {
		dm.log.Warn("can't write space hash", zap.Error(err))
	}
}

func (dm *DiffManager) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	if req.DiffType == spacesyncproto.DiffType_V2 {
		return HandleRangeRequest(ctx, dm.diffContainer.NewDiff(), req)
	} else {
		return HandleRangeRequest(ctx, dm.diffContainer.OldDiff(), req)
	}
}

func (dm *DiffManager) AllIds() []string {
	return dm.diffContainer.NewDiff().Ids()
}
