package settingsdocument

import "sync"

type settingsQueue struct {
	sync.Mutex
	queued  map[string]struct{}
	deleted map[string]struct{}
}

func newSettingsQueue() *settingsQueue {
	return &settingsQueue{
		Mutex:   sync.Mutex{},
		queued:  map[string]struct{}{},
		deleted: map[string]struct{}{},
	}
}

func (q *settingsQueue) add(ids []string) {
	q.Lock()
	defer q.Unlock()
	for _, id := range ids {
		if _, exists := q.deleted[id]; exists {
			continue
		}
		if _, exists := q.queued[id]; exists {
			continue
		}
		q.queued[id] = struct{}{}
	}
}

func (q *settingsQueue) getQueued() (ids []string) {
	q.Lock()
	defer q.Unlock()
	ids = make([]string, 0, len(q.queued))
	for id := range q.queued {
		ids = append(ids, id)
	}
	return
}

func (q *settingsQueue) delete(id string) {
	q.Lock()
	defer q.Unlock()
	delete(q.queued, id)
	q.deleted[id] = struct{}{}
}

func (q *settingsQueue) queueIfDeleted(id string) {
	q.Lock()
	defer q.Unlock()
	if _, exists := q.deleted[id]; exists {
		delete(q.deleted, id)
		q.queued[id] = struct{}{}
	}
}

func (q *settingsQueue) exists(id string) bool {
	q.Lock()
	defer q.Unlock()
	if _, exists := q.deleted[id]; exists {
		return true
	}
	if _, exists := q.queued[id]; exists {
		return true
	}
	return false
}
