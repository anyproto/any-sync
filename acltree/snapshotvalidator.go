package acltree

import (
	"fmt"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
)

type snapshotValidator struct {
	aclTree      *Tree
	identity     string
	key          keys.EncryptionPrivKey
	decoder      keys.SigningPubKeyDecoder
	stateBuilder *aclStateBuilder
}

func newSnapshotValidator(
	decoder keys.SigningPubKeyDecoder,
	accountData *account.AccountData) *snapshotValidator {
	return &snapshotValidator{
		identity:     accountData.Identity,
		key:          accountData.EncKey,
		decoder:      decoder,
		stateBuilder: newACLStateBuilder(decoder, accountData),
	}
}

func (s *snapshotValidator) Init(aclTree *Tree) error {
	s.aclTree = aclTree
	return s.stateBuilder.Init(aclTree)
}

func (s *snapshotValidator) ValidateSnapshot(ch *Change) (bool, error) {
	st, found, err := s.stateBuilder.BuildBefore(ch.Id)
	if err != nil {
		return false, err
	}

	if !found {
		return false, fmt.Errorf("didn't find snapshot in ACL Tree")
	}

	otherSt, err := newACLStateFromSnapshotChange(ch.Content, s.identity, s.key, s.decoder)
	if err != nil {
		return false, err
	}

	return st.equal(otherSt), nil
}
