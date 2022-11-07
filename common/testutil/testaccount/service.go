package testaccount

import (
	accountService "github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
)

// AccountTestService provides service for test purposes, generates new random account every Init
type AccountTestService struct {
	acc *account.AccountData
}

func (s *AccountTestService) Init(a *app.App) (err error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return
	}
	ident, err := signKey.GetPublic().Raw()
	if err != nil {
		return
	}
	s.acc = &account.AccountData{
		Identity: ident,
		SignKey:  signKey,
		EncKey:   encKey,
	}
	return nil
}

func (s *AccountTestService) Name() (name string) {
	return accountService.CName
}

func (s *AccountTestService) Account() *account.AccountData {
	return s.acc
}
