package nodeconf

import (
	commonaccount "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/go-chash"
	"go.uber.org/zap"
	"sync"
)

const CName = "common.nodeconf"

const (
	PartitionCount    = 3000
	ReplicationFactor = 3
)

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type Service interface {
	NodeConf
	app.Component
}

type service struct {
	accountId string
	last      NodeConf
	mu        sync.RWMutex
}

func (s *service) Init(a *app.App) (err error) {
	nodesConf := a.MustComponent("config").(ConfigGetter)
	s.accountId = a.MustComponent(commonaccount.CName).(commonaccount.Service).Account().PeerId
	return s.setLastConfiguration(nodesConf.GetNodeConf())
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) setLastConfiguration(c Configuration) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last != nil && s.last.Id() == c.Id {
		return
	}

	nc := &nodeConf{
		id:        c.Id,
		c:         c,
		accountId: s.accountId,
		addrs:     map[string][]string{},
	}
	if nc.chash, err = chash.New(chash.Config{
		PartitionCount:    PartitionCount,
		ReplicationFactor: ReplicationFactor,
	}); err != nil {
		return
	}

	members := make([]chash.Member, 0, len(c.Nodes))
	for _, n := range c.Nodes {
		if n.HasType(NodeTypeTree) {
			members = append(members, n)
		}
		if n.HasType(NodeTypeConsensus) {
			nc.consensusPeers = append(nc.consensusPeers, n.PeerId)
		}
		if n.HasType(NodeTypeFile) {
			nc.filePeers = append(nc.filePeers, n.PeerId)
		}
		if n.HasType(NodeTypeCoordinator) {
			nc.coordinatorPeers = append(nc.coordinatorPeers, n.PeerId)
		}
		nc.allMembers = append(nc.allMembers, n)
		nc.addrs[n.PeerId] = n.Addresses
	}
	if err = nc.chash.AddMembers(members...); err != nil {
		return
	}
	var beforeId = ""
	if s.last != nil {
		beforeId = s.last.Id()
	}
	log.Info("nodeConf changed", zap.String("before", beforeId), zap.String("after", nc.Id()))
	s.last = nc
	return
}

func (s *service) Id() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.Id()
}

func (s *service) Configuration() Configuration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.Configuration()
}

func (s *service) NodeIds(spaceId string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.NodeIds(spaceId)
}

func (s *service) IsResponsible(spaceId string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.IsResponsible(spaceId)
}

func (s *service) FilePeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.FilePeers()
}

func (s *service) ConsensusPeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.ConsensusPeers()
}

func (s *service) CoordinatorPeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.CoordinatorPeers()
}

func (s *service) PeerAddresses(peerId string) ([]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.PeerAddresses(peerId)
}

func (s *service) CHash() chash.CHash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.CHash()
}

func (s *service) Partition(spaceId string) (part int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.Partition(spaceId)
}

func (s *service) NodeTypes(nodeId string) []NodeType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.NodeTypes(nodeId)
}
