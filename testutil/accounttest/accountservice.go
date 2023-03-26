package accounttest

import (
	accountService "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/anytypeio/any-sync/util/peer"
)

// AccountTestService provides service for test purposes, generates new random account every Init
type AccountTestService struct {
	acc *accountdata.AccountKeys
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

	peerId, err := peer.IdFromSigningPubKey(peerKey.GetPublic())
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

func (s *AccountTestService) NodeConf(addrs []string) nodeconf.NodeConfig {
	return nodeconf.NodeConfig{
		PeerId:    s.acc.PeerId,
		Addresses: addrs,
		Types:     []nodeconf.NodeType{nodeconf.NodeTypeTree},
	}
}
