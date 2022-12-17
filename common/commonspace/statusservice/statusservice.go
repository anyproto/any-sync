package statusservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
	"golang.org/x/exp/slices"
	"sync"
)

var log = logger.NewNamed("commonspace.statusservice")

type StatusService interface {
	HeadsChange(treeId string, heads []string)
	HeadsReceive(senderId, treeId string, heads []string)
	Watch(treeId string, ch chan bool)
	Unwatch(treeId string)
	StateCounter() uint64
	RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64)
}

type statusEntry struct {
	head         string
	stateCounter uint64
}

type statusService struct {
	sync.Mutex
	spaceId       string
	treeHeads     map[string]statusEntry
	watchers      map[string]chan bool
	configuration nodeconf.Configuration
	stateCounter  uint64
}

func NewStatusService(spaceId string, configuration nodeconf.Configuration) StatusService {
	return &statusService{
		spaceId:       spaceId,
		treeHeads:     map[string]statusEntry{},
		watchers:      map[string]chan bool{},
		configuration: configuration,
		stateCounter:  0,
	}
}

func (s *statusService) HeadsChange(treeId string, heads []string) {
	s.Lock()
	defer s.Unlock()
	s.treeHeads[treeId] = statusEntry{
		head:         heads[0],
		stateCounter: s.stateCounter,
	}
	if watcher, ok := s.watchers[treeId]; ok {
		select {
		case watcher <- false:
		default:
		}
	}
	s.stateCounter++
}

func (s *statusService) HeadsReceive(senderId, treeId string, heads []string) {
	s.Lock()
	defer s.Unlock()
	curHead, ok := s.treeHeads[treeId]
	if !ok {
		return
	}
	if len(heads) == 0 || !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) {
		return
	}
	if slice.FindPos(heads, curHead.head) == -1 {
		return
	}
	delete(s.treeHeads, treeId)
	if watcher, ok := s.watchers[treeId]; ok {
		select {
		case watcher <- true:
		default:
		}
	}
}

func (s *statusService) Watch(treeId string, ch chan bool) {
	s.Lock()
	defer s.Unlock()
	s.watchers[treeId] = ch
}

func (s *statusService) Unwatch(treeId string) {
	s.Lock()
	defer s.Unlock()
	delete(s.watchers, treeId)
}

func (s *statusService) StateCounter() uint64 {
	s.Lock()
	defer s.Unlock()
	return s.stateCounter
}

func (s *statusService) RemoveAllExcept(senderId string, differentRemoteIds []string, stateCounter uint64) {
	if !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) {
		return
	}
	s.Lock()
	defer s.Unlock()
	slices.Sort(differentRemoteIds)
	for treeId, entry := range s.treeHeads {
		if entry.stateCounter > stateCounter {
			continue
		}
		if _, found := slices.BinarySearch(differentRemoteIds, treeId); !found {
			delete(s.treeHeads, treeId)
		}
	}
}
