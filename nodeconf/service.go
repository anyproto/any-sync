//go:generate mockgen -destination mock_nodeconf/mock_nodeconf.go github.com/anyproto/any-sync/nodeconf Service
package nodeconf

import (
	"context"
	"errors"
	"sync"

	"github.com/anyproto/go-chash"
	"go.uber.org/zap"

	commonaccount "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/util/periodicsync"
)

const CName = "common.nodeconf"

const (
	PartitionCount    = 3000
	ReplicationFactor = 3
)

var log = logger.NewNamed(CName)

type NetworkCompatibilityStatus int

const (
	NetworkCompatibilityStatusUnknown NetworkCompatibilityStatus = iota
	NetworkCompatibilityStatusOk
	NetworkCompatibilityStatusError
	NetworkCompatibilityStatusIncompatible
	NetworkCompatibilityStatusNeedsUpdate
)

func New() Service {
	return new(service)
}

type Service interface {
	NodeConf
	NetworkCompatibilityStatus() NetworkCompatibilityStatus
	app.ComponentRunnable
}

type NetworkProtoVersionChecker interface {
	IsNetworkNeedsUpdate(ctx context.Context) (bool, error)
}

type service struct {
	accountId string
	config    Configuration
	source    Source
	store     Store
	last      NodeConf
	mu        sync.RWMutex
	sync      periodicsync.PeriodicSync

	compatibilityStatus        NetworkCompatibilityStatus
	networkProtoVersionChecker NetworkProtoVersionChecker
}

func mergedTreeNodes(configNodes []Node, lastStoredNodes []Node) (mergedAddrs []string) {
	treeNodes := make(map[string]int)
	mustRewriteNods := false

	for _, node := range lastStoredNodes {
		if node.HasType(NodeTypeCoordinator) {
			for _, addr := range node.Addresses {
				treeNodes[addr] = 1
			}
		}
	}

	for _, node := range configNodes {
		if node.HasType(NodeTypeCoordinator) {
			for _, addr := range node.Addresses {
				if _, found := treeNodes[addr]; !found {
					mustRewriteNods = true
				}
				treeNodes[addr] += 1
			}
		}
	}

	if mustRewriteNods {
		mergedAddrs = make([]string, 0)
		for addr, _ := range treeNodes {
			mergedAddrs = append(mergedAddrs, addr)
		}

		return
	}

	return nil

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

	if mergedAddrs := mergedTreeNodes(s.config.Nodes, lastStored.Nodes); mergedAddrs != nil {
		for _, node := range lastStored.Nodes {
			if node.HasType(NodeTypeCoordinator) {
				node.Addresses = mergedAddrs
				break // can we break here? node structure in yaml allows to have multiple sections with the same nodeType
			}
		}
	}

	var updatePeriodSec = 600
	if confUpd, ok := a.MustComponent("config").(ConfigUpdateGetter); ok && confUpd.GetNodeConfUpdateInterval() > 0 {
		updatePeriodSec = confUpd.GetNodeConfUpdateInterval()
	}

	s.sync = periodicsync.NewPeriodicSync(updatePeriodSec, 0, func(ctx context.Context) (err error) {
		err = s.updateConfiguration(ctx)
		if err != nil {
			if err == ErrConfigurationNotChanged || err == ErrConfigurationNotFound {
				err = nil
			}
		}
		return
	}, log)
	s.networkProtoVersionChecker = app.MustComponent[NetworkProtoVersionChecker](a)
	return s.setLastConfiguration(lastStored)
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(_ context.Context) (err error) {
	s.sync.Run()
	return
}

func (s *service) NetworkCompatibilityStatus() NetworkCompatibilityStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.compatibilityStatus
}

func (s *service) updateConfiguration(ctx context.Context) (err error) {
	last, err := s.source.GetLast(ctx, s.Configuration().Id)
	if err != nil {
		if errors.Is(err, ErrConfigurationNotChanged) {
			err = s.updateCompatibilityStatus(ctx)
			return err
		}
		s.setCompatibilityStatusByErr(err)
		return err
	}

	if err = s.updateCompatibilityStatus(ctx); err != nil {
		return err
	}

	if err = s.saveAndSetLastConfiguration(ctx, last); err != nil {
		return err
	}

	return nil
}

func (s *service) updateCompatibilityStatus(ctx context.Context) error {
	needsUpdate, checkErr := s.networkProtoVersionChecker.IsNetworkNeedsUpdate(ctx)
	if checkErr != nil {
		return checkErr
	}
	if needsUpdate {
		s.setCompatibilityStatus(NetworkCompatibilityStatusNeedsUpdate)
	} else {
		s.setCompatibilityStatus(NetworkCompatibilityStatusOk)
	}
	return nil
}

func (s *service) saveAndSetLastConfiguration(ctx context.Context, last Configuration) error {
	if err := s.store.SaveLast(ctx, last); err != nil {
		return err
	}

	if err := s.setLastConfiguration(last); err != nil {
		return err
	}

	return nil
}

func (s *service) setCompatibilityStatus(status NetworkCompatibilityStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compatibilityStatus = status
}

func (s *service) setCompatibilityStatusByErr(err error) {
	var status NetworkCompatibilityStatus

	switch err {
	case nil:
		status = NetworkCompatibilityStatusOk
	case handshake.ErrIncompatibleVersion:
		status = NetworkCompatibilityStatusIncompatible
	case net.ErrUnableToConnect:
		status = NetworkCompatibilityStatusUnknown
	default:
		status = NetworkCompatibilityStatusError
	}

	s.setCompatibilityStatus(status)
}

func (s *service) setLastConfiguration(c Configuration) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last != nil && s.last.Id() == c.Id {
		return
	}

	nc, err := сonfigurationToNodeConf(c)
	if err != nil {
		return
	}
	nc.accountId = s.accountId

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

func (s *service) NamingNodePeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.NamingNodePeers()
}

func (s *service) PaymentProcessingNodePeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last.PaymentProcessingNodePeers()
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
	if s.sync != nil {
		s.sync.Close()
	}
	return
}
