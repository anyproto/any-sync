package syncstatus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/periodicsync"
	"github.com/anytypeio/any-sync/util/slice"
	"golang.org/x/exp/slices"
)

const (
	syncUpdateInterval = 5
	syncTimeout        = time.Second
)

var log = logger.NewNamed("common.commonspace.syncstatus")

type UpdateReceiver interface {
	UpdateTree(ctx context.Context, treeId string, status SyncStatus) (err error)
	UpdateNodeConnection(online bool)
}

type StatusUpdater interface {
	HeadsChange(treeId string, heads []string)
	HeadsReceive(senderId, treeId string, heads []string)

	SetNodesOnline(senderId string, online bool)
	StateCounter() uint64
	RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64)

	Run()
	Close() error
}

type StatusWatcher interface {
	Watch(treeId string) (err error)
	Unwatch(treeId string)
	SetUpdateReceiver(updater UpdateReceiver)
}

type StatusProvider interface {
	StatusUpdater
	StatusWatcher
}

type SyncStatus int

const (
	StatusUnknown SyncStatus = iota
	StatusSynced
	StatusNotSynced
)

type treeHeadsEntry struct {
	heads        []string
	stateCounter uint64
	syncStatus   SyncStatus
}

type treeStatus struct {
	treeId string
	status SyncStatus
	heads  []string
}

type syncStatusProvider struct {
	sync.Mutex
	configuration  nodeconf.NodeConf
	periodicSync   periodicsync.PeriodicSync
	updateReceiver UpdateReceiver
	storage        spacestorage.SpaceStorage

	spaceId      string
	treeHeads    map[string]treeHeadsEntry
	watchers     map[string]struct{}
	stateCounter uint64
	nodesOnline  bool

	treeStatusBuf []treeStatus

	updateIntervalSecs int
	updateTimeout      time.Duration
}

type SyncStatusDeps struct {
	UpdateIntervalSecs int
	UpdateTimeout      time.Duration
	Configuration      nodeconf.NodeConf
	Storage            spacestorage.SpaceStorage
}

func DefaultDeps(configuration nodeconf.NodeConf, store spacestorage.SpaceStorage) SyncStatusDeps {
	return SyncStatusDeps{
		UpdateIntervalSecs: syncUpdateInterval,
		UpdateTimeout:      syncTimeout,
		Configuration:      configuration,
		Storage:            store,
	}
}

func NewSyncStatusProvider(spaceId string, deps SyncStatusDeps) StatusProvider {
	return &syncStatusProvider{
		spaceId:            spaceId,
		treeHeads:          map[string]treeHeadsEntry{},
		watchers:           map[string]struct{}{},
		updateIntervalSecs: deps.UpdateIntervalSecs,
		updateTimeout:      deps.UpdateTimeout,
		configuration:      deps.Configuration,
		storage:            deps.Storage,
		stateCounter:       0,
	}
}

func (s *syncStatusProvider) SetUpdateReceiver(updater UpdateReceiver) {
	s.Lock()
	defer s.Unlock()

	s.updateReceiver = updater
}

func (s *syncStatusProvider) Run() {
	s.periodicSync = periodicsync.NewPeriodicSync(
		s.updateIntervalSecs,
		s.updateTimeout,
		s.update,
		log)
	s.periodicSync.Run()
}

func (s *syncStatusProvider) HeadsChange(treeId string, heads []string) {
	s.Lock()
	defer s.Unlock()

	var headsCopy []string
	headsCopy = append(headsCopy, heads...)

	s.treeHeads[treeId] = treeHeadsEntry{
		heads:        headsCopy,
		stateCounter: s.stateCounter,
		syncStatus:   StatusNotSynced,
	}
	s.stateCounter++
}

func (s *syncStatusProvider) SetNodesOnline(senderId string, online bool) {
	if !s.isSenderResponsible(senderId) {
		return
	}

	s.Lock()
	defer s.Unlock()

	s.nodesOnline = online
}

func (s *syncStatusProvider) update(ctx context.Context) (err error) {
	s.treeStatusBuf = s.treeStatusBuf[:0]

	s.Lock()
	if s.updateReceiver == nil {
		s.Unlock()
		return
	}
	for treeId := range s.watchers {
		// that means that we haven't yet got the status update
		treeHeads, exists := s.treeHeads[treeId]
		if !exists {
			err = fmt.Errorf("treeHeads should always exist for watchers")
			s.Unlock()
			return
		}
		s.treeStatusBuf = append(s.treeStatusBuf, treeStatus{treeId, treeHeads.syncStatus, treeHeads.heads})
	}
	s.Unlock()
	s.updateReceiver.UpdateNodeConnection(s.nodesOnline)
	for _, entry := range s.treeStatusBuf {
		err = s.updateReceiver.UpdateTree(ctx, entry.treeId, entry.status)
		if err != nil {
			return
		}
	}
	return
}

func (s *syncStatusProvider) HeadsReceive(senderId, treeId string, heads []string) {
	s.Lock()
	defer s.Unlock()

	curTreeHeads, ok := s.treeHeads[treeId]
	if !ok || curTreeHeads.syncStatus == StatusSynced {
		return
	}

	// checking if other node is responsible
	if len(heads) == 0 || !s.isSenderResponsible(senderId) {
		return
	}

	// checking if we received the head that we are interested in
	for _, head := range heads {
		if idx, found := slices.BinarySearch(curTreeHeads.heads, head); found {
			curTreeHeads.heads[idx] = ""
		}
	}
	curTreeHeads.heads = slice.DiscardFromSlice(curTreeHeads.heads, func(h string) bool {
		return h == ""
	})
	if len(curTreeHeads.heads) == 0 {
		curTreeHeads.syncStatus = StatusSynced
	}
	s.treeHeads[treeId] = curTreeHeads
}

func (s *syncStatusProvider) Watch(treeId string) (err error) {
	s.Lock()
	defer s.Unlock()
	_, ok := s.treeHeads[treeId]
	if !ok {
		var (
			st    treestorage.TreeStorage
			heads []string
		)
		st, err = s.storage.TreeStorage(treeId)
		if err != nil {
			return
		}
		heads, err = st.Heads()
		if err != nil {
			return
		}
		slices.Sort(heads)
		s.stateCounter++
		s.treeHeads[treeId] = treeHeadsEntry{
			heads:        heads,
			stateCounter: s.stateCounter,
			syncStatus:   StatusUnknown,
		}
	}

	s.watchers[treeId] = struct{}{}
	return
}

func (s *syncStatusProvider) Unwatch(treeId string) {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.watchers[treeId]; ok {
		delete(s.watchers, treeId)
	}
}

func (s *syncStatusProvider) Close() (err error) {
	s.periodicSync.Close()
	return
}

func (s *syncStatusProvider) StateCounter() uint64 {
	s.Lock()
	defer s.Unlock()

	return s.stateCounter
}

func (s *syncStatusProvider) RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64) {
	// if sender is not a responsible node, then this should have no effect
	if !s.isSenderResponsible(senderId) {
		return
	}

	s.Lock()
	defer s.Unlock()

	slices.Sort(differentRemoteIds)
	for treeId, entry := range s.treeHeads {
		// if the current update is outdated
		if entry.stateCounter > stateCounter {
			continue
		}
		// if we didn't find our treeId in heads ids which are different from us and node
		if _, found := slices.BinarySearch(differentRemoteIds, treeId); !found {
			entry.syncStatus = StatusSynced
			s.treeHeads[treeId] = entry
		}
	}
}

func (s *syncStatusProvider) isSenderResponsible(senderId string) bool {
	return slices.Contains(s.configuration.NodeIds(s.spaceId), senderId)
}
