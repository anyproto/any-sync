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
	return st.storage.IterateEntries(ctx, headstorage.IterOpts{Deleted: true}, func(entry headstorage.HeadsEntry) (bool, error) {
		switch entry.DeletedStatus {
		case headstorage.DeletedStatusQueued:
			st.queued[entry.Id] = struct{}{}
		case headstorage.DeletedStatusDeleted:
			st.deleted[entry.Id] = struct{}{}
		default:
		}
		return true, nil
	})
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
