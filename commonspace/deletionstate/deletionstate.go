//go:generate mockgen -destination mock_deletionstate/mock_deletionstate.go github.com/anyproto/any-sync/commonspace/deletionstate ObjectDeletionState
package deletionstate

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"go.uber.org/zap"
	"sync"
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

type objectDeletionState struct {
	sync.RWMutex
	log                  logger.CtxLogger
	queued               map[string]struct{}
	deleted              map[string]struct{}
	stateUpdateObservers []StateUpdateObserver
	storage              spacestorage.SpaceStorage
}

func (st *objectDeletionState) Init(a *app.App) (err error) {
	st.storage = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
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

		var status string
		status, err := st.storage.TreeDeletedStatus(id)
		if err != nil {
			st.log.Warn("failed to get deleted status", zap.String("treeId", id), zap.Error(err))
			continue
		}

		switch status {
		case spacestorage.TreeDeletedStatusQueued:
			st.queued[id] = struct{}{}
		case spacestorage.TreeDeletedStatusDeleted:
			st.deleted[id] = struct{}{}
		default:
			err := st.storage.SetTreeDeletedStatus(id, spacestorage.TreeDeletedStatusQueued)
			if err != nil {
				st.log.Warn("failed to set deleted status", zap.String("treeId", id), zap.Error(err))
				continue
			}
			st.queued[id] = struct{}{}
		}
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

func (st *objectDeletionState) Delete(id string) (err error) {
	st.Lock()
	defer st.Unlock()
	delete(st.queued, id)
	st.deleted[id] = struct{}{}
	err = st.storage.SetTreeDeletedStatus(id, spacestorage.TreeDeletedStatusDeleted)
	if err != nil {
		return
	}
	return
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
