package accounttest

import (
	accountService "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
)

// AccountTestService provides service for test purposes, generates new random account every Init
type AccountTestService struct {
	acc *accountdata.AccountData
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
	s.acc = &accountdata.AccountData{
		Identity: ident,
		SignKey:  signKey,
		EncKey:   encKey,
	}
	return nil
}

func (s *AccountTestService) Name() (name string) {
	return accountService.CName
}

func (s *AccountTestService) Account() *accountdata.AccountData {
	return s.acc
}
