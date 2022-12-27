package nodeconf

import (
	commonaccount "github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
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
	nodesConf := a.MustComponent("config").(configGetter).GetNodes()
	s.accountId = a.MustComponent(commonaccount.CName).(commonaccount.Service).Account().PeerId

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
		if n.PeerId == s.accountId {
			continue
		}
		if n.HasType(NodeTypeConsensus) {
			fileConfig.consensusPeers = append(fileConfig.consensusPeers, n.PeerId)
		}
		if n.HasType(NodeTypeFile) {
			fileConfig.filePeers = append(fileConfig.filePeers, n.PeerId)
		}
		fileConfig.allMembers = append(fileConfig.allMembers, n)
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

func nodeFromConfigNode(n NodeConfig) (*Node, error) {
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
		Addresses:     n.Addresses,
		PeerId:        n.PeerId,
		SigningKey:    decodedSigningKey.GetPublic(),
		EncryptionKey: decodedEncryptionKey.GetPublic(),
	}, nil
}
