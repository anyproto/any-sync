package node

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"go.uber.org/zap"
)

const CName = "NodesService"

var log = logger.NewNamed("nodesservice")

type Node struct {
	Address             string
	PeerId              string
	SigningKey          signingkey.PubKey
	EncryptionKey       encryptionkey.PubKey
	SigningKeyString    string
	EncryptionKeyString string
}

func (n *Node) Id() string {
	return n.PeerId
}

func (n *Node) Capacity() float64 {
	return 1
}

func New() app.Component {
	return &service{}
}

type Service interface {
	Nodes() []*Node
}

type service struct {
	nodes []*Node
}

func (s *service) Init(a *app.App) (err error) {
	cfg := a.MustComponent(config.CName).(*config.Config)
	signDecoder := signingkey.NewEDPrivKeyDecoder()
	rsaDecoder := encryptionkey.NewRSAPrivKeyDecoder()

	var filteredNodes []*Node
	for _, n := range cfg.Nodes {
		if n.PeerId == cfg.Account.PeerId {
			continue
		}
		node, err := nodeFromConfigNode(n, n.PeerId, signDecoder, rsaDecoder)
		if err != nil {
			return err
		}
		log.With(zap.String("node", node.PeerId)).Debug("adding peer to known nodes")
		filteredNodes = append(filteredNodes, node)
	}
	s.nodes = filteredNodes
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) Nodes() []*Node {
	return s.nodes
}

func nodeFromConfigNode(
	n config.Node,
	peerId string) (*Node, error) {
	decodedSigningKey, err := privateSigningDecoder.DecodeFromString(n.SigningKey)
	if err != nil {
		return nil, err
	}

	decodedEncryptionKey, err := privateEncryptionDecoder.DecodeFromString(n.EncryptionKey)
	if err != nil {
		return nil, err
	}

	encKeyString, err := privateEncryptionDecoder.EncodeToString(decodedEncryptionKey.(encryptionkey.PrivKey).GetPublic())
	if err != nil {
		return nil, err
	}

	signKeyString, err := privateSigningDecoder.EncodeToString(decodedSigningKey.(signingkey.PrivKey).GetPublic())

	return &Node{
		Address:             n.Address,
		PeerId:              peerId,
		SigningKey:          decodedSigningKey.(signingkey.PrivKey).GetPublic(),
		EncryptionKey:       decodedEncryptionKey.(encryptionkey.PrivKey).GetPublic(),
		SigningKeyString:    signKeyString,
		EncryptionKeyString: encKeyString,
	}, nil
}
