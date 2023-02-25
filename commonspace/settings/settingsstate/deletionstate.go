//go:generate mockgen -destination mock_settingsstate/mock_settingsstate.go github.com/anytypeio/any-sync/commonspace/settings/settingsstate ObjectDeletionState,StateBuilder,ChangeFactory
package settingsstate

import (
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"sync"
)

type StateUpdateObserver func(ids []string)

type ObjectDeletionState interface {
	AddObserver(observer StateUpdateObserver)
	Add(ids []string) (err error)
	GetQueued() (ids []string)
	Delete(id string) (err error)
	Exists(id string) bool
	FilterJoin(ids ...[]string) (filtered []string)
}

type objectDeletionState struct {
	sync.RWMutex
	queued               map[string]struct{}
	deleted              map[string]struct{}
	stateUpdateObservers []StateUpdateObserver
	storage              spacestorage.SpaceStorage
}

func NewObjectDeletionState(storage spacestorage.SpaceStorage) ObjectDeletionState {
	return &objectDeletionState{
		queued:  map[string]struct{}{},
		deleted: map[string]struct{}{},
		storage: storage,
	}
}

func (st *objectDeletionState) AddObserver(observer StateUpdateObserver) {
	st.Lock()
	defer st.Unlock()
	st.stateUpdateObservers = append(st.stateUpdateObservers, observer)
}

func (st *objectDeletionState) Add(ids []string) (err error) {
	st.Lock()
	defer func() {
		st.Unlock()
		if err != nil {
			return
		}
		for _, ob := range st.stateUpdateObservers {
			ob(ids)
		}
	}()

	for _, id := range ids {
		if _, exists := st.deleted[id]; exists {
			continue
		}
		if _, exists := st.queued[id]; exists {
			continue
		}

		var status string
		status, err = st.storage.TreeDeletedStatus(id)
		if err != nil {
			return
		}

		switch status {
		case spacestorage.TreeDeletedStatusQueued:
			st.queued[id] = struct{}{}
		case spacestorage.TreeDeletedStatusDeleted:
			st.deleted[id] = struct{}{}
		default:
			st.queued[id] = struct{}{}
			err = st.storage.SetTreeDeletedStatus(id, spacestorage.TreeDeletedStatusQueued)
			if err != nil {
				return
			}
		}
	}
	return
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

func (st *objectDeletionState) FilterJoin(ids ...[]string) (filtered []string) {
	st.RLock()
	defer st.RUnlock()
	filter := func(ids []string) {
		for _, id := range ids {
			if !st.exists(id) {
				filtered = append(filtered, id)
			}
		}
	}
	for _, arr := range ids {
		filter(arr)
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
