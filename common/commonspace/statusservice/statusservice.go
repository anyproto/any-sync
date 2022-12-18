package statusservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/periodicsync"
	"golang.org/x/exp/slices"
	"sync"
	"time"
)

const (
	statusServiceUpdateInterval = 5
	statusServiceTimeout        = time.Second
)

var log = logger.NewNamed("commonspace.statusservice")

type Updater func(ctx context.Context, treeId string, status bool) (err error)

type StatusService interface {
	HeadsChange(treeId string, heads []string)
	HeadsReceive(senderId, treeId string, heads []string)
	Watch(treeId string)
	Unwatch(treeId string)
	StateCounter() uint64
	RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64)

	SetUpdater(updater Updater)
	Run()
	Close() error
}

type statusEntry struct {
	head         string
	stateCounter uint64
}

type treeStatus struct {
	treeId string
	status bool
}

type statusService struct {
	sync.Mutex
	configuration nodeconf.Configuration
	periodicSync  periodicsync.PeriodicSync
	updater       Updater

	spaceId      string
	treeHeads    map[string]statusEntry
	watchers     map[string]struct{}
	stateCounter uint64
	closed       bool

	treeStatusBuf []treeStatus
}

func NewStatusService(spaceId string, configuration nodeconf.Configuration) StatusService {
	return &statusService{
		spaceId:       spaceId,
		treeHeads:     map[string]statusEntry{},
		watchers:      map[string]struct{}{},
		configuration: configuration,
		stateCounter:  0,
	}
}

func (s *statusService) SetUpdater(updater Updater) {
	s.Lock()
	defer s.Unlock()

	s.updater = updater
}

func (s *statusService) Run() {
	s.periodicSync = periodicsync.NewPeriodicSync(
		statusServiceUpdateInterval,
		statusServiceTimeout,
		s.update,
		log)
	s.periodicSync.Run()
}

func (s *statusService) HeadsChange(treeId string, heads []string) {
	s.Lock()
	defer s.Unlock()

	// TODO: save to storage
	s.treeHeads[treeId] = statusEntry{
		head:         heads[0],
		stateCounter: s.stateCounter,
	}
	s.stateCounter++
}

func (s *statusService) update(ctx context.Context) (err error) {
	s.treeStatusBuf = s.treeStatusBuf[:0]

	s.Lock()
	if s.updater == nil {
		s.Unlock()
		return
	}
	for treeId := range s.watchers {
		// that means that we haven't yet got the status update
		_, exists := s.treeHeads[treeId]
		s.treeStatusBuf = append(s.treeStatusBuf, treeStatus{treeId, !exists})
	}
	s.Unlock()

	for _, entry := range s.treeStatusBuf {
		err = s.updater(ctx, entry.treeId, entry.status)
		if err != nil {
			return
		}
	}
	return
}

func (s *statusService) HeadsReceive(senderId, treeId string, heads []string) {
	s.Lock()
	defer s.Unlock()

	curHead, ok := s.treeHeads[treeId]
	if !ok {
		return
	}
	// checking if other node is responsible
	if len(heads) == 0 || !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) {
		return
	}
	// checking if we received the head that we are interested in
	if !slices.Contains(heads, curHead.head) {
		return
	}

	// TODO: save to storage
	delete(s.treeHeads, treeId)
}

func (s *statusService) Watch(treeId string) {
	s.Lock()
	defer s.Unlock()

	s.watchers[treeId] = struct{}{}
}

func (s *statusService) Unwatch(treeId string) {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.watchers[treeId]; ok {
		delete(s.watchers, treeId)
	}
}

func (s *statusService) Close() (err error) {
	s.periodicSync.Close()
	return
}

func (s *statusService) StateCounter() uint64 {
	s.Lock()
	defer s.Unlock()

	return s.stateCounter
}

func (s *statusService) RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64) {
	// if sender is not a responsible node, then this should have no effect
	if !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) {
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
			delete(s.treeHeads, treeId)
		}
	}
}
