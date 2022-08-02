package node

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

const CName = "NodesService"

type Node struct {
	Address             string
	PeerId              string
	SigningKey          signingkey.PubKey
	EncryptionKey       encryptionkey.PubKey
	SigningKeyString    string
	EncryptionKeyString string
}

func NewFromFile(path string) (app.Component, error) {
	nodeInfo := &config.NodeInfo{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(data, nodeInfo); err != nil {
		return nil, err
	}

	var nodes []*Node
	privateEncryptionDecoder := encryptionkey.NewRSAPrivKeyDecoder()
	privateSigningDecoder := signingkey.NewEDPrivKeyDecoder()
	for _, n := range nodeInfo.Nodes {
		// ignoring ourselves
		if n.Alias == nodeInfo.CurrentAlias {
			continue
		}
		newNode, err := nodeFromYamlNode(n, privateSigningDecoder, privateEncryptionDecoder)
		if err != nil {
			return nil, fmt.Errorf("failed to parse node: %w", err)
		}
		nodes = append(nodes, newNode)
	}

	return &service{nodes: nodes}, nil
}

type Service interface {
	Nodes() []*Node
}

type service struct {
	nodes []*Node
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
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

func nodeFromYamlNode(
	n *config.Node,
	privateSigningDecoder keys.Decoder,
	privateEncryptionDecoder keys.Decoder) (*Node, error) {
	decodedSigningKey, err := privateSigningDecoder.DecodeFromString(n.SigningKey)
	if err != nil {
		return nil, err
	}

	decodedEncryptionKey, err := privateEncryptionDecoder.DecodeFromString(n.EncryptionKey)
	if err != nil {
		return nil, err
	}

	rawSigning, err := decodedSigningKey.Raw()
	if err != nil {
		return nil, err
	}

	libp2pKey, err := crypto.UnmarshalEd25519PrivateKey(rawSigning)
	if err != nil {
		return nil, err
	}

	peerId, err := peer.IDFromPublicKey(libp2pKey.GetPublic())
	if err != nil {
		return nil, err
	}

	encKeyString, err := privateEncryptionDecoder.EncodeToString(
		decodedEncryptionKey.(encryptionkey.PrivKey).GetPublic())
	if err != nil {
		return nil, err
	}

	signKeyString, err := privateSigningDecoder.EncodeToString(
		decodedSigningKey.(signingkey.PrivKey).GetPublic())

	return &Node{
		Address:             n.Address,
		PeerId:              peerId.String(),
		SigningKey:          decodedSigningKey.(signingkey.PrivKey).GetPublic(),
		EncryptionKey:       decodedEncryptionKey.(encryptionkey.PrivKey).GetPublic(),
		SigningKeyString:    signKeyString,
		EncryptionKeyString: encKeyString,
	}, nil
}
