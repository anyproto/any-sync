//go:generate mockgen -destination mock_deletionstate/mock_deletionstate.go github.com/anyproto/any-sync/commonspace/deletionstate ObjectDeletionState
package deletionstate

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
)

var log = logger.NewNamed(CName)

const CName = "common.commonspace.deletionstate"

type StateUpdateObserver func(ids []string)

type ObjectDeletionState interface {
	app.Component
	AddObserver(observer StateUpdateObserver)
	Add(ids map[string]struct{})
	GetQueued() (ids []string)
	Delete(id string) (err error)
	Exists(id string) bool
	Filter(ids []string) (filtered []string)
}

const setTimeout = 5 * time.Second

type objectDeletionState struct {
	sync.RWMutex
	log                  logger.CtxLogger
	queued               map[string]struct{}
	deleted              map[string]struct{}
	stateUpdateObservers []StateUpdateObserver
	storage              headstorage.HeadStorage
}

func (st *objectDeletionState) Run(ctx context.Context) (err error) {
	err = st.storage.IterateEntries(ctx, headstorage.IterOpts{Deleted: true}, func(entry headstorage.HeadsEntry) (bool, error) {
		switch entry.DeletedStatus {
		case headstorage.DeletedStatusQueued:
			st.queued[entry.Id] = struct{}{}
		case headstorage.DeletedStatusDeleted:
			st.deleted[entry.Id] = struct{}{}
		default:
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	// Scan for orphaned children of fully deleted parents.
	// This handles the case where an old client deleted a parent without
	// cascading to bound children. After upgrade, this queues them for cleanup.
	if len(st.deleted) == 0 {
		return nil
	}
	var orphans []string
	if len(st.deleted) <= orphanQueryMaxDeleted {
		orphans = st.orphansByParent(ctx)
	} else {
		orphans = st.orphansByScan(ctx)
	}
	// update after the reads are done: the scan holds an iterator open
	for _, childId := range orphans {
		if uErr := st.updateStatus(childId, headstorage.DeletedStatusQueued); uErr != nil {
			st.log.Warn("failed to queue orphaned child", zap.String("childId", childId), zap.Error(uErr))
			continue
		}
		st.queued[childId] = struct{}{}
	}
	return nil
}

// up to this many deleted ids Run queries the children of each one: that cost stays bounded
// (a query costs about as much as scanning ten entries), the number of entries with a parent doesn't
var orphanQueryMaxDeleted = 100

// orphansByParent queries the children of every deleted id
func (st *objectDeletionState) orphansByParent(ctx context.Context) (orphans []string) {
	for parentId := range st.deleted {
		children, cErr := st.storage.GetEntriesByParentId(ctx, parentId)
		if cErr != nil {
			continue
		}
		for _, child := range children {
			if child.DeletedStatus == headstorage.DeletedStatusNotDeleted {
				orphans = append(orphans, child.Id)
			}
		}
	}
	return
}

// orphansByScan makes one pass over the entries with a parent and matches them against the deleted ids
func (st *objectDeletionState) orphansByScan(ctx context.Context) (orphans []string) {
	cErr := st.storage.IterateEntriesWithParent(ctx, func(child headstorage.HeadsEntry) (bool, error) {
		if child.DeletedStatus != headstorage.DeletedStatusNotDeleted {
			return true, nil
		}
		if _, parentDeleted := st.deleted[child.ParentId]; parentDeleted {
			orphans = append(orphans, child.Id)
		}
		return true, nil
	})
	if cErr != nil {
		st.log.Warn("failed to scan for orphaned children", zap.Error(cErr))
	}
	return
}

func (st *objectDeletionState) Close(ctx context.Context) (err error) {
	return nil
}

func (st *objectDeletionState) Init(a *app.App) (err error) {
	st.storage = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage).HeadStorage()
	return nil
}

func (st *objectDeletionState) Name() (name string) {
	return CName
}

func New() ObjectDeletionState {
	return &objectDeletionState{
		log:     log,
		queued:  map[string]struct{}{},
		deleted: map[string]struct{}{},
	}
}

func (st *objectDeletionState) AddObserver(observer StateUpdateObserver) {
	st.Lock()
	defer st.Unlock()
	st.stateUpdateObservers = append(st.stateUpdateObservers, observer)
}

func (st *objectDeletionState) Add(ids map[string]struct{}) {
	var added []string
	st.Lock()
	defer func() {
		st.Unlock()
		for _, ob := range st.stateUpdateObservers {
			ob(added)
		}
	}()

	for id := range ids {
		if _, exists := st.deleted[id]; exists {
			continue
		}
		if _, exists := st.queued[id]; exists {
			continue
		}
		err := st.updateStatus(id, headstorage.DeletedStatusQueued)
		if err != nil {
			st.log.Warn("failed to set deleted status", zap.String("treeId", id), zap.Error(err))
			continue
		}
		st.queued[id] = struct{}{}
		added = append(added, id)
	}
}

func (st *objectDeletionState) GetQueued() (ids []string) {
	st.RLock()
	defer st.RUnlock()
	ids = make([]string, 0, len(st.queued))
	for id := range st.queued {
		ids = append(ids, id)
	}
	return
}

func (st *objectDeletionState) updateStatus(id string, status headstorage.DeletedStatus) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), setTimeout)
	defer cancel()
	return st.storage.UpdateEntry(ctx, headstorage.HeadsUpdate{
		Id:            id,
		DeletedStatus: &status,
	})
}

func (st *objectDeletionState) Delete(id string) (err error) {
	st.Lock()
	defer st.Unlock()
	delete(st.queued, id)
	st.deleted[id] = struct{}{}
	return st.updateStatus(id, headstorage.DeletedStatusDeleted)
}

func (st *objectDeletionState) Exists(id string) bool {
	st.RLock()
	defer st.RUnlock()
	return st.exists(id)
}

func (st *objectDeletionState) Filter(ids []string) (filtered []string) {
	st.RLock()
	defer st.RUnlock()
	for _, id := range ids {
		if !st.exists(id) {
			filtered = append(filtered, id)
		}
	}
	return
}

func (st *objectDeletionState) exists(id string) bool {
	if _, exists := st.deleted[id]; exists {
		return true
	}
	if _, exists := st.queued[id]; exists {
		return true
	}
	return false
}
