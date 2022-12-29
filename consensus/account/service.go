package account

import (
	commonaccount "github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/accountdata"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
)

type service struct {
	accountData *accountdata.AccountData
	peerId      string
}

func (s *service) Account() *accountdata.AccountData {
	return s.accountData
}

func New() app.Component {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	acc := a.MustComponent("config").(commonaccount.ConfigGetter).GetAccount()

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

	s.accountData = &accountdata.AccountData{
		Identity: identity,
		SignKey:  decodedSigningKey,
		EncKey:   decodedEncryptionKey,
	}
	s.peerId = acc.PeerId

	return nil
}

func (s *service) Name() (name string) {
	return commonaccount.CName
}
