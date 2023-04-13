package nodeconf

import (
	"context"
	commonaccount "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/go-chash"
	"go.uber.org/zap"
	"sync"
	"time"
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
	app.ComponentRunnable
}

type service struct {
	accountId string
	config    Configuration
	source    Source
	store     Store
	last      NodeConf
	mu        sync.RWMutex
}

func (s *service) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(ConfigGetter).GetNodeConf()
	s.accountId = a.MustComponent(commonaccount.CName).(commonaccount.Service).Account().PeerId
	s.source = a.MustComponent(CNameSource).(Source)
	s.store = a.MustComponent(CNameStore).(Store)
	lastStored, err := s.store.GetLast(context.Background(), s.config.NetworkId)
	if err == ErrConfigurationNotFound {
		lastStored = s.config
		err = nil
	}
	return s.setLastConfiguration(lastStored)
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(_ context.Context) (err error) {
	go s.updateLoop(context.Background())
	return
}

func (s *service) updateLoop(ctx context.Context) {
	for _ = range time.NewTicker(time.Minute * 10).C {
		err := s.updateConfiguration(ctx)
		if err != nil {
			if err == ErrConfigurationNotChanged {
				continue
			}
			log.Info("can't update configuration", zap.Error(err))
		}
	}
}

func (s *service) updateConfiguration(ctx context.Context) (err error) {
	last, err := s.source.GetLast(ctx, s.Configuration().Id)
	if err != nil {
		return
	}
	if err = s.store.SaveLast(ctx, last); err != nil {
		return
	}
	return s.setLastConfiguration(last)
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
	if s.last != nil {
		log.Info("net configuration changed", zap.String("before", beforeId), zap.String("after", nc.Id()))
	} else {
		log.Info("net configuration applied", zap.String("netId", nc.Configuration().NetworkId), zap.String("id", nc.Id()))
	}
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

func (s *service) Close(ctx context.Context) (err error) {
	return
}
