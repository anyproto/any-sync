package api

import "go.uber.org/zap"

type watcher struct {
	spaceId     string
	treeId      string
	watcher     chan bool
	watcherDone chan struct{}
}

func newWatcher(spaceId, treeId string, ch chan bool) *watcher {
	return &watcher{
		spaceId:     spaceId,
		treeId:      treeId,
		watcher:     ch,
		watcherDone: make(chan struct{}),
	}
}

func (w *watcher) run() {
	log := log.With(zap.String("spaceId", w.spaceId), zap.String("treeId", w.treeId))
	log.Debug("started watching")
	defer close(w.watcherDone)
	for {
		synced, ok := <-w.watcher
		if !ok {
			log.Debug("stopped watching")
			return
		}
		log.With(zap.Bool("synced", synced)).Debug("updated sync status")
	}
}

func (w *watcher) close() {
	<-w.watcherDone
}
