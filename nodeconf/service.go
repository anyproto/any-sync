package nodeconf

import (
	commonaccount "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/util/keys"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-chash"
	"github.com/libp2p/go-libp2p/core/peer"
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
	GetLast() Configuration
	SetLastConfig(id string, nodes []NodeConfig) (err error)
	app.Component
}

type service struct {
	accountId string
	last      Configuration
	mu        sync.RWMutex
}

type Node struct {
	Addresses     []string
	PeerId        string
	SigningKey    signingkey.PubKey
	EncryptionKey encryptionkey.PubKey
}

func (n *Node) Id() string {
	return n.PeerId
}

func (n *Node) Capacity() float64 {
	return 1
}

func (s *service) Init(a *app.App) (err error) {
	nodesConf := a.MustComponent("config").(ConfigGetter)
	s.accountId = a.MustComponent(commonaccount.CName).(commonaccount.Service).Account().PeerId
	return s.SetLastConfig(nodesConf.GetNodesConfId(), nodesConf.GetNodes())
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) GetLast() Configuration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last
}

func (s *service) SetLastConfig(id string, nodesConf []NodeConfig) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last != nil && s.last.Id() == id {
		return
	}

	fileConfig := &configuration{
		id:        id,
		accountId: s.accountId,
	}
	if fileConfig.chash, err = chash.New(chash.Config{
		PartitionCount:    PartitionCount,
		ReplicationFactor: ReplicationFactor,
	}); err != nil {
		return
	}

	members := make([]chash.Member, 0, len(nodesConf))
	for _, n := range nodesConf {
		if n.HasType(NodeTypeTree) {
			var member *Node
			member, err = nodeFromConfigNode(n)
			if err != nil {
				return
			}
			members = append(members, member)
		}
		if n.HasType(NodeTypeConsensus) {
			fileConfig.consensusPeers = append(fileConfig.consensusPeers, n.PeerId)
		}
		if n.HasType(NodeTypeFile) {
			fileConfig.filePeers = append(fileConfig.filePeers, n.PeerId)
		}
		if n.HasType(NodeTypeCoordinator) {
			fileConfig.coordinatorPeers = append(fileConfig.coordinatorPeers, n.PeerId)
		}
		fileConfig.allMembers = append(fileConfig.allMembers, n)
	}
	if err = fileConfig.chash.AddMembers(members...); err != nil {
		return
	}
	var beforeId = ""
	if s.last != nil {
		beforeId = s.last.Id()
	}
	log.Info("configuration changed", zap.String("before", beforeId), zap.String("after", fileConfig.Id()))
	s.last = fileConfig
	return
}

func nodeFromConfigNode(n NodeConfig) (*Node, error) {
	p, err := peer.Decode(n.PeerId)
	if err != nil {
		return nil, err
	}
	ic, err := p.ExtractPublicKey()
	if err != nil {
		return nil, err
	}

	icRaw, err := ic.Raw()
	if err != nil {
		return nil, err
	}

	sigPubKey, err := signingkey.UnmarshalEd25519PublicKey(icRaw)
	if err != nil {
		return nil, err
	}

	encPubKey, err := keys.DecodeKeyFromString(
		n.EncryptionKey,
		encryptionkey.NewEncryptionRsaPubKeyFromBytes,
		nil)
	if err != nil {
		return nil, err
	}

	return &Node{
		Addresses:     n.Addresses,
		PeerId:        n.PeerId,
		SigningKey:    sigPubKey,
		EncryptionKey: encPubKey,
	}, nil
}
