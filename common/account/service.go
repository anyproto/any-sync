package account

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

const CName = "account"

type Service interface {
	Account() *account.AccountData
}

type service struct {
	accountData *account.AccountData
	peerId      string
}

func (s *service) Account() *account.AccountData {
	return s.accountData
}

type StaticAccount struct {
	SigningKey    string `yaml:"signingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
}

func New() app.Component {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	cfg := a.MustComponent(config.CName).(*config.Config)
	// TODO: add deviceKey
	acc := cfg.Account

	decodedEncryptionKey, err := keys.DecodeKeyFromString(
		acc.EncryptionKey,
		encryptionkey.NewEncryptionRsaPrivKeyFromBytes,
		nil)
	if err != nil {
		return err
	}

	decodedSigningKey, err := keys.DecodeKeyFromString(
		acc.SigningKey,
		signingkey.NewSigningEd25519PrivKeyFromBytes,
		nil)
	if err != nil {
		return err
	}

	identity, err := decodedSigningKey.GetPublic().Raw()
	if err != nil {
		return err
	}

	s.accountData = &account.AccountData{
		Identity: identity,
		SignKey:  decodedSigningKey,
		EncKey:   decodedEncryptionKey,
	}
	s.peerId = acc.PeerId

	return nil
}

func (s *service) Name() (name string) {
	return CName
}
