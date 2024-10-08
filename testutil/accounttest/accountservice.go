package accounttest

import (
	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/util/crypto"
)

// AccountTestService provides service for test purposes, generates new random account every Init
type AccountTestService struct {
	acc *accountdata.AccountKeys
}

func NewWithAcc(acc *accountdata.AccountKeys) *AccountTestService {
	return &AccountTestService{acc: acc}
}

func (s *AccountTestService) Init(a *app.App) (err error) {
	if s.acc != nil {
		return
	}
	signKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return
	}

	peerKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return err
	}

	peerId, err := crypto.IdFromSigningPubKey(peerKey.GetPublic())
	if err != nil {
		return err
	}
	s.acc = &accountdata.AccountKeys{
		PeerKey: peerKey,
		SignKey: signKey,
		PeerId:  peerId.String(),
	}
	return nil
}

func (s *AccountTestService) Name() (name string) {
	return accountService.CName
}

func (s *AccountTestService) Account() *accountdata.AccountKeys {
	return s.acc
}
