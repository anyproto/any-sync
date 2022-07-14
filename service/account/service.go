package account

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
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
	SigningKey    string `yaml:"siginingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
}

func NewFromFile(path string) (app.Component, error) {
	acc := &StaticAccount{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(data, acc); err != nil {
		return nil, err
	}
	privateEncryptionDecoder := keys.NewKeyDecoder(func(bytes []byte) (keys.Key, error) {
		return encryptionkey.NewEncryptionRsaPrivKeyFromBytes(bytes)
	})
	privateSigningDecoder := keys.NewKeyDecoder(func(bytes []byte) (keys.Key, error) {
		return signingkey.NewSigningEd25519PrivKeyFromBytes(bytes)
	})
	// TODO: Convert this to new decoder
	publicSigningDecoder := signingkey.NewEd25519PubKeyDecoder()

	decodedEncryptionKey, err := privateEncryptionDecoder.DecodeFromString(acc.EncryptionKey)
	if err != nil {
		return nil, err
	}
	decodedSiginingKey, err := privateSigningDecoder.DecodeFromString(acc.EncryptionKey)
	if err != nil {
		return nil, err
	}
	signKey := decodedSiginingKey.(signingkey.PrivKey)
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
