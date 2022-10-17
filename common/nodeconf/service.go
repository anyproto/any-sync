package nodeconf

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	config2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	encryptionkey2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	signingkey2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-chash"
)

const CName = "common.nodeconf"

const (
	partitionCount    = 3000
	replicationFactor = 3
)

var log = logger.NewNamed(CName)

type Service interface {
	GetLast() Configuration
	GetById(id string) Configuration
	ConsensusPeers() []string
	app.Component
}

type service struct {
	accountId string

	consensusPeers []string
	last           Configuration
}

type Node struct {
	Address       string
	PeerId        string
	SigningKey    signingkey2.PubKey
	EncryptionKey encryptionkey2.PubKey
}

func (n *Node) Id() string {
	return n.PeerId
}

func (n *Node) Capacity() float64 {
	return 1
}

func (s *service) Init(a *app.App) (err error) {
	conf := a.MustComponent(config2.CName).(*config2.Config)
	s.accountId = conf.Account.PeerId

	config := &configuration{
		id:        "config",
		accountId: s.accountId,
	}
	if config.chash, err = chash.New(chash.Config{
		PartitionCount:    partitionCount,
		ReplicationFactor: replicationFactor,
	}); err != nil {
		return
	}
	members := make([]chash.Member, 0, len(conf.Nodes))
	for _, n := range conf.Nodes {
		if n.IsConsensus {
			s.consensusPeers = append(s.consensusPeers, n.PeerId)
			continue
		}
		var member *Node
		member, err = nodeFromConfigNode(n)
		if err != nil {
			return
		}
		members = append(members, member)
	}
	if err = config.chash.AddMembers(members...); err != nil {
		return
	}
	s.last = config
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

func (s *service) ConsensusPeers() []string {
	return s.consensusPeers
}

func nodeFromConfigNode(
	n config2.Node) (*Node, error) {
	decodedSigningKey, err := keys.DecodeKeyFromString(
		n.SigningKey,
		signingkey2.UnmarshalEd25519PrivateKey,
		nil)
	if err != nil {
		return nil, err
	}

	decodedEncryptionKey, err := keys.DecodeKeyFromString(
		n.EncryptionKey,
		encryptionkey2.NewEncryptionRsaPrivKeyFromBytes,
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
