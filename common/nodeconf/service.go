package nodeconf

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-chash"
)

const CName = "common.nodeconf"

const (
	partitionCount    = 3000
	replicationFactor = 3
)

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type Service interface {
	GetLast() Configuration
	GetById(id string) Configuration
	app.Component
}

type service struct {
	accountId string
	last      Configuration
}

type Node struct {
	Address       string
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
	conf := a.MustComponent(config.CName).(*config.Config)
	s.accountId = conf.Account.PeerId

	fileConfig := &configuration{
		id:        "config",
		accountId: s.accountId,
	}
	if fileConfig.chash, err = chash.New(chash.Config{
		PartitionCount:    partitionCount,
		ReplicationFactor: replicationFactor,
	}); err != nil {
		return
	}
	members := make([]chash.Member, 0, len(conf.Nodes))
	for _, n := range conf.Nodes {
		if n.HasType(config.NodeTypeTree) {
			var member *Node
			member, err = nodeFromConfigNode(n)
			if err != nil {
				return
			}
			members = append(members, member)
		}
		if n.PeerId == s.accountId {
			continue
		}
		if n.HasType(config.NodeTypeConsensus) {
			fileConfig.consensusPeers = append(fileConfig.consensusPeers, n.PeerId)
		}
		if n.HasType(config.NodeTypeFile) {
			fileConfig.filePeers = append(fileConfig.filePeers, n.PeerId)
		}
	}
	if err = fileConfig.chash.AddMembers(members...); err != nil {
		return
	}
	s.last = fileConfig
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) GetLast() Configuration {
	return s.last
}

func (s *service) GetById(id string) Configuration {
	//TODO implement me
	panic("implement me")
}

func nodeFromConfigNode(
	n config.Node) (*Node, error) {
	decodedSigningKey, err := keys.DecodeKeyFromString(
		n.SigningKey,
		signingkey.UnmarshalEd25519PrivateKey,
		nil)
	if err != nil {
		return nil, err
	}

	decodedEncryptionKey, err := keys.DecodeKeyFromString(
		n.EncryptionKey,
		encryptionkey.NewEncryptionRsaPrivKeyFromBytes,
		nil)
	if err != nil {
		return nil, err
	}

	return &Node{
		Address:       n.Address,
		PeerId:        n.PeerId,
		SigningKey:    decodedSigningKey.GetPublic(),
		EncryptionKey: decodedEncryptionKey.GetPublic(),
	}, nil
}
