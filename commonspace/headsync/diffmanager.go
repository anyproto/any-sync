package headsync

import (
	"context"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

// DiffManager holds the current diff (DiffType_V3).
//
// Upgrading the diff algorithm to a next DiffType requires a coexistence
// period, see how V2/V3 lived together before V2 removal (git history up to
// this commit): a DiffContainer with old/new diffs filled by DiffManager,
// dual hashes in statestorage, a switch by req.DiffType in HandleRangeRequest
// and by resp.DiffType in remote.DiffTypeCheck, so that the newest common
// type wins. The old type can be dropped once the secureservice
// compatibleVersions exclude all peers that don't support the new one.
//
// Note for that future upgrade: V3-only responders (this code and the node's
// HeadSync handler) reject unknown diff types with an error instead of
// answering with their best supported type, so a V(n+1) requester must treat
// such an error as "type unsupported" and fall back to requesting V(n).
type DiffManager struct {
	diff          ldiff.Diff
	storage       spacestorage.SpaceStorage
	syncAcl       syncacl.SyncAcl
	log           logger.CtxLogger
	ctx           context.Context
	deletionState deletionstate.ObjectDeletionState
}

func NewDiffManager(
	diff ldiff.Diff,
	storage spacestorage.SpaceStorage,
	syncAcl syncacl.SyncAcl,
	log logger.CtxLogger,
	ctx context.Context,
	deletionState deletionstate.ObjectDeletionState,
) *DiffManager {
	return &DiffManager{
		diff:          diff,
		storage:       storage,
		syncAcl:       syncAcl,
		log:           log,
		ctx:           ctx,
		deletionState: deletionState,
	}
}

func (dm *DiffManager) FillDiff(ctx context.Context) error {
	els := make([]ldiff.Element, 0, 100)
	hasher := ldiff.NewHasher()
	defer ldiff.ReleaseHasher(hasher)
	err := dm.storage.HeadStorage().IterateEntries(ctx, headstorage.IterOpts{}, func(entry headstorage.HeadsEntry) (bool, error) {
		// skip empty roots, except non-derived ones without a common snapshot:
		// those have historically been part of the diff and must stay to keep
		// the space hash stable (UpdateHeads deliberately differs, see there)
		if len(entry.Heads) > 0 && entry.Heads[0] == entry.Id && (entry.IsDerived || entry.CommonSnapshot != "") {
			return true, nil
		}
		els = append(els, ldiff.Element{
			Id:   entry.Id,
			Head: hasher.HashId(concatStrings(entry.Heads)),
		})
		return true, nil
	})
	if err != nil {
		return err
	}
	log.Debug("setting acl", zap.String("aclId", dm.syncAcl.Id()), zap.String("headId", dm.syncAcl.Head().Id))
	if len(els) > 0 {
		dm.diff.Set(els...)
	}
	if err := dm.storage.StateStorage().SetHash(ctx, dm.diff.Hash()); err != nil {
		dm.log.Error("can't write space hash", zap.Error(err))
		return err
	}
	return nil
}

func (dm *DiffManager) TryDiff(ctx context.Context, rdiff RemoteDiff) (newIds, changedIds, removedIds []string, err error) {
	needsSync, err := rdiff.DiffTypeCheck(ctx, dm.diff)
	err = rpcerr.Unwrap(err)
	if err != nil {
		return nil, nil, nil, err
	}
	if needsSync {
		newIds, changedIds, removedIds, err = dm.diff.Diff(ctx, rdiff)
		err = rpcerr.Unwrap(err)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return newIds, changedIds, removedIds, nil
}

func (dm *DiffManager) UpdateHeads(update headstorage.HeadsEntry) {
	if update.DeletedStatus != headstorage.DeletedStatusNotDeleted {
		_ = dm.diff.RemoveId(update.Id)
	} else {
		if dm.deletionState.Exists(update.Id) {
			return
		}
		// live updates never add empty roots; unlike FillDiff there is no
		// common-snapshot exception here — this asymmetry predates the V2
		// removal and both conditions must stay as is, or every existing
		// space hash changes
		if len(update.Heads) == 1 && update.Heads[0] == update.Id {
			return
		}
		hasher := ldiff.NewHasher()
		defer ldiff.ReleaseHasher(hasher)
		dm.diff.Set(ldiff.Element{
			Id:   update.Id,
			Head: hasher.HashId(concatStrings(update.Heads)),
		})
	}
	err := dm.storage.StateStorage().SetHash(dm.ctx, dm.diff.Hash())
	if err != nil {
		dm.log.Warn("can't write space hash", zap.Error(err))
	}
}

func (dm *DiffManager) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	if req.DiffType != spacesyncproto.DiffType_V3 {
		return nil, spacesyncproto.ErrUnexpected
	}
	return HandleRangeRequest(ctx, dm.diff, req)
}

func (dm *DiffManager) AllIds() []string {
	return dm.diff.Ids()
}
