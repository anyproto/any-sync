package accounttest

import (
	accountService "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/keys"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/any-sync/util/peer"
)

// AccountTestService provides service for test purposes, generates new random account every Init
type AccountTestService struct {
	acc *accountdata.AccountData
}

func (s *AccountTestService) Init(a *app.App) (err error) {
	if s.acc != nil {
		return
	}
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

	peerKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return err
	}

	peerId, err := peer.IdFromSigningPubKey(peerKey.GetPublic())
	if err != nil {
		return err
	}
	s.acc = &accountdata.AccountData{
		Identity: ident,
		PeerKey:  peerKey,
		SignKey:  signKey,
		EncKey:   encKey,
		PeerId:   peerId.String(),
	}
	return nil
}

func (s *AccountTestService) Name() (name string) {
	return accountService.CName
}

func (s *AccountTestService) Account() *accountdata.AccountData {
	return s.acc
}

func (s *AccountTestService) NodeConf(addrs []string) nodeconf.NodeConfig {
	encEk, err := keys.EncodeKeyToString(s.acc.EncKey.GetPublic())
	if err != nil {
		panic(err)
	}
	return nodeconf.NodeConfig{
		PeerId:        s.acc.PeerId,
		Addresses:     addrs,
		EncryptionKey: encEk,
		Types:         []nodeconf.NodeType{nodeconf.NodeTypeTree},
	}
}
