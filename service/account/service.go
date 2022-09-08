package account

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
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

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	cfg := a.MustComponent(config.CName).(*config.Config)

	// decoding our keys
	privateEncryptionDecoder := encryptionkey.NewRSAPrivKeyDecoder()
	privateSigningDecoder := signingkey.NewEDPrivKeyDecoder()
	publicSigningDecoder := signingkey.NewEDPubKeyDecoder()
	acc := cfg.Account

	decodedEncryptionKey, err := privateEncryptionDecoder.DecodeFromString(acc.EncryptionKey)
	if err != nil {
		return err
	}
	decodedSigningKey, err := privateSigningDecoder.DecodeFromString(acc.SigningKey)
	if err != nil {
		return err
	}
	signKey := decodedSigningKey.(signingkey.PrivKey)
	identity, err := publicSigningDecoder.EncodeToString(signKey.GetPublic())
	if err != nil {
		return err
	}

	// TODO: using acl lib format basically, but we should simplify this
	s.accountData = &account.AccountData{
		Identity: identity,
		SignKey:  signKey,
		EncKey:   decodedEncryptionKey.(encryptionkey.PrivKey),
		Decoder:  signingkey.NewEDPubKeyDecoder(),
	}
	s.peerId = acc.PeerId

	return nil
}

func (s *service) Name() (name string) {
	return CName
}
