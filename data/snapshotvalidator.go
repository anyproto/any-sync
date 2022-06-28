package data

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"
)

type SnapshotValidator struct {
	aclTree  *Tree
	identity string
	key      threadmodels.EncryptionPrivKey
	decoder  threadmodels.SigningPubKeyDecoder
}

func NewSnapshotValidator(
	aclTree *Tree,
	identity string,
	key threadmodels.EncryptionPrivKey,
	decoder threadmodels.SigningPubKeyDecoder) *SnapshotValidator {
	return &SnapshotValidator{
		aclTree:  aclTree,
		identity: identity,
		key:      key,
		decoder:  decoder,
	}
}

func (s *SnapshotValidator) ValidateSnapshot(ch *Change) (bool, error) {
	stateBuilder, err := NewACLStateBuilder(s.aclTree, s.identity, s.key, s.decoder)
	if err != nil {
		return false, err
	}

	st, found, err := stateBuilder.BuildBefore(ch.Id)
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
