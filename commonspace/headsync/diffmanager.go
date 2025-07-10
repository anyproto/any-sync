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
	var (
		commonEls           = make([]ldiff.Element, 0, 100)
		onlyOldEls          = make([]ldiff.Element, 0, 100)
		noCommonSnapshotEls = make([]ldiff.Element, 0, 100)
	)
	err := dm.storage.HeadStorage().IterateEntries(ctx, headstorage.IterOpts{}, func(entry headstorage.HeadsEntry) (bool, error) {
		// empty derived roots shouldn't be set in all hashes
		if entry.IsDerived && entry.Heads[0] == entry.Id {
			return true, nil
		}
		// empty roots shouldn't be set in new hashe
		if entry.Heads[0] == entry.Id {
			onlyOldEls = append(onlyOldEls, ldiff.Element{
				Id:   entry.Id,
				Head: concatStrings(entry.Heads),
			})
			return true, nil
		}
		if entry.CommonSnapshot != "" {
			commonEls = append(commonEls, ldiff.Element{
				Id:   entry.Id,
				Head: concatStrings(entry.Heads),
			})
		} else {
			noCommonSnapshotEls = append(noCommonSnapshotEls, ldiff.Element{
				Id:   entry.Id,
				Head: concatStrings(entry.Heads),
			})
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	log.Debug("setting acl", zap.String("aclId", dm.syncAcl.Id()), zap.String("headId", dm.syncAcl.Head().Id))
	hasher := ldiff.NewHasher()
	dm.setHeadsForNewDiff(commonEls, noCommonSnapshotEls, hasher)
	dm.setHeadsForOldDiff(commonEls, onlyOldEls, noCommonSnapshotEls, hasher)
	ldiff.ReleaseHasher(hasher)
	oldHash := dm.diffContainer.OldDiff().Hash()
	newHash := dm.diffContainer.NewDiff().Hash()
	if err := dm.storage.StateStorage().SetHash(ctx, oldHash, newHash); err != nil {
		dm.log.Error("can't write space hash", zap.Error(err))
		return err
	}
	return nil
}

func (dm *DiffManager) setHeadsForNewDiff(commonEls, noCommonSnapshotEls []ldiff.Element, hasher *ldiff.Hasher) {
	for _, el := range append(commonEls, noCommonSnapshotEls...) {
		hash := hasher.HashId(el.Head)
		dm.diffContainer.NewDiff().Set(ldiff.Element{
			Id:   el.Id,
			Head: hash,
		})
	}
}

func (dm *DiffManager) setHeadsForOldDiff(commonEls, onlyOldEls, noCommonSnapshotEls []ldiff.Element, hasher *ldiff.Hasher) {
	for _, el := range append(commonEls, onlyOldEls...) {
		hash := hasher.HashId(el.Head)
		dm.diffContainer.OldDiff().Set(ldiff.Element{
			Id:   el.Id,
			Head: hash,
		})
	}
	for _, el := range noCommonSnapshotEls {
		dm.diffContainer.OldDiff().Set(el)
	}
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
		hasher := ldiff.NewHasher()
		defer ldiff.ReleaseHasher(hasher)
		if len(update.Heads) == 1 && update.Heads[0] == update.Id {
			dm.diffContainer.OldDiff().Set(ldiff.Element{
				Id:   update.Id,
				Head: hasher.HashId(update.Heads[0]),
			})
			return
		}
		concatHeads := concatStrings(update.Heads)
		if update.Id == dm.keyValue.DefaultStore().Id() {
			dm.diffContainer.NewDiff().Set(ldiff.Element{
				Id:   update.Id,
				Head: hasher.HashId(concatHeads),
			})
			dm.diffContainer.OldDiff().Set(ldiff.Element{
				Id:   update.Id,
				Head: concatHeads,
			})
		} else {
			for _, diff := range []ldiff.Diff{dm.diffContainer.OldDiff(), dm.diffContainer.NewDiff()} {
				diff.Set(ldiff.Element{
					Id:   update.Id,
					Head: hasher.HashId(concatHeads),
				})
			}
		}
	}
	oldHash := dm.diffContainer.OldDiff().Hash()
	newHash := dm.diffContainer.NewDiff().Hash()
	err := dm.storage.StateStorage().SetHash(dm.ctx, oldHash, newHash)
	if err != nil {
		dm.log.Warn("can't write space hash", zap.Error(err))
	}
}

func (dm *DiffManager) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	switch req.DiffType {
	case spacesyncproto.DiffType_V3:
		return HandleRangeRequest(ctx, dm.diffContainer.NewDiff(), req)
	case spacesyncproto.DiffType_V2:
		return HandleRangeRequest(ctx, dm.diffContainer.NewDiff(), req)
	default:
		return nil, spacesyncproto.ErrUnexpected
	}
}

func (dm *DiffManager) AllIds() []string {
	return dm.diffContainer.NewDiff().Ids()
}
