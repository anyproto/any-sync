package deletionstate

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"sync"
)

type StateUpdateObserver func(ids []string)

type DeletionState struct {
	sync.RWMutex
	queued               map[string]struct{}
	deleted              map[string]struct{}
	stateUpdateObservers []StateUpdateObserver
	storage              storage.SpaceStorage
}

func NewDeletionState(storage storage.SpaceStorage) *DeletionState {
	return &DeletionState{
		queued:  map[string]struct{}{},
		deleted: map[string]struct{}{},
		storage: storage,
	}
}

func (st *DeletionState) AddObserver(observer StateUpdateObserver) {
	st.Lock()
	defer st.Unlock()
	st.stateUpdateObservers = append(st.stateUpdateObservers, observer)
}

func (st *DeletionState) Add(ids []string) (err error) {
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
		case storage.TreeDeletedStatusQueued:
			st.queued[id] = struct{}{}
		case storage.TreeDeletedStatusDeleted:
			st.deleted[id] = struct{}{}
		default:
			st.queued[id] = struct{}{}
			err = st.storage.SetTreeDeletedStatus(id, storage.TreeDeletedStatusQueued)
			if err != nil {
				return
			}
		}
	}
	return
}

func (st *DeletionState) GetQueued() (ids []string) {
	st.RLock()
	defer st.RUnlock()
	ids = make([]string, 0, len(st.queued))
	for id := range st.queued {
		ids = append(ids, id)
	}
	return
}

func (st *DeletionState) Delete(id string) (err error) {
	st.Lock()
	defer st.Unlock()
	delete(st.queued, id)
	st.deleted[id] = struct{}{}
	err = st.storage.SetTreeDeletedStatus(id, storage.TreeDeletedStatusQueued)
	if err != nil {
		return
	}
	return
}

func (st *DeletionState) Exists(id string) bool {
	st.RLock()
	defer st.RUnlock()
	return st.exists(id)
}

func (st *DeletionState) FilterJoin(ids ...[]string) (filtered []string) {
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

func (st *DeletionState) exists(id string) bool {
	if _, exists := st.deleted[id]; exists {
		return true
	}
	if _, exists := st.queued[id]; exists {
		return true
	}
	return false
}
