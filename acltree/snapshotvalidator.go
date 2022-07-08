package acltree

import (
	"fmt"
)

type SnapshotValidator struct {
	aclTree      *Tree
	identity     string
	key          threadmodels.EncryptionPrivKey
	decoder      threadmodels.SigningPubKeyDecoder
	stateBuilder *ACLStateBuilder
}

func NewSnapshotValidator(
	decoder threadmodels.SigningPubKeyDecoder,
	accountData *AccountData) *SnapshotValidator {
	return &SnapshotValidator{
		identity:     accountData.Identity,
		key:          accountData.EncKey,
		decoder:      decoder,
		stateBuilder: NewACLStateBuilder(decoder, accountData),
	}
}

func (s *SnapshotValidator) Init(aclTree *Tree) error {
	s.aclTree = aclTree
	return s.stateBuilder.Init(aclTree)
}

func (s *SnapshotValidator) ValidateSnapshot(ch *Change) (bool, error) {
	st, found, err := s.stateBuilder.BuildBefore(ch.Id)
	if err != nil {
		return false, err
	}

	if !found {
		return false, fmt.Errorf("didn't find snapshot in ACL tree")
	}

	otherSt, err := NewACLStateFromSnapshot(ch.Content.GetAclData().GetAclSnapshot(), s.identity, s.key, s.decoder)
	if err != nil {
		return false, err
	}

	return st.Equal(otherSt), nil
}
