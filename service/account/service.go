package account

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

const CName = "account"

type Service interface {
	Account() *account.AccountData
}

type service struct {
	accountData *account.AccountData
}

func (s *service) Account() *account.AccountData {
	return s.accountData
}

type StaticAccount struct {
	SigningKey    string `yaml:"signingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
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
	privateEncryptionDecoder := encryptionkey.NewRSAPrivKeyDecoder()
	privateSigningDecoder := signingkey.NewEDPrivKeyDecoder()
	publicSigningDecoder := signingkey.NewEDPubKeyDecoder()

	var acc *config.Node
	for _, node := range nodeInfo.Nodes {
		if node.Alias == nodeInfo.CurrentAlias {
			acc = node
			break
		}
	}
	if acc == nil {
		return nil, fmt.Errorf("the node should have a defined alias")
	}

	decodedEncryptionKey, err := privateEncryptionDecoder.DecodeFromString(acc.EncryptionKey)
	if err != nil {
		return nil, err
	}
	decodedSigningKey, err := privateSigningDecoder.DecodeFromString(acc.SigningKey)
	if err != nil {
		return nil, err
	}
	signKey := decodedSigningKey.(signingkey.PrivKey)
	identity, err := publicSigningDecoder.EncodeToString(signKey.GetPublic())
	if err != nil {
		return nil, err
	}

	accountData := &account.AccountData{
		Identity: identity,
		SignKey:  signKey,
		EncKey:   decodedEncryptionKey.(encryptionkey.PrivKey),
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}
	return &service{accountData: accountData}, nil
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	return nil
}

func (s *service) Name() (name string) {
	return CName
}
