package tree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

type aclStateBuilder struct {
	tree     *Tree
	identity string
	key      encryptionkey.PrivKey
	decoder  signingkey.PubKeyDecoder
}

func newACLStateBuilderWithIdentity(decoder signingkey.PubKeyDecoder, accountData *account.AccountData) *aclStateBuilder {
	return &aclStateBuilder{
		decoder:  decoder,
		identity: accountData.Identity,
		key:      accountData.EncKey,
	}
}

func newACLStateBuilder() *aclStateBuilder {
	return &aclStateBuilder{}
}

func (sb *aclStateBuilder) Init(tree *Tree) error {
	sb.tree = tree
	return nil
}

func (sb *aclStateBuilder) Build() (*ACLState, error) {
	state, _, err := sb.BuildBefore("")
	return state, err
}

func (sb *aclStateBuilder) BuildBefore(beforeId string) (*ACLState, bool, error) {
	var (
		err         error
		startChange = sb.tree.root
		state       *ACLState
		foundId     = false
	)

	if sb.decoder != nil {
		state = newACLStateWithIdentity(sb.identity, sb.key, sb.decoder)
	} else {
		state = newACLState()
	}

	if beforeId == startChange.Id {
		return state, true, nil
	}

	iterFunc := func(c *Change) (isContinue bool) {
		defer func() {
			if err == nil {
				startChange = c
			} else if err != ErrDocumentForbidden {
				log.Errorf("marking change %s as invalid: %v", c.Id, err)
				sb.tree.RemoveInvalidChange(c.Id)
			}
		}()
		err = state.applyChangeAndUpdate(c)
		if err != nil {
			return false
		}

		// the user can't make changes
		if !state.hasPermission(c.Content.Identity, aclpb.ACLChange_Writer) && !state.hasPermission(c.Content.Identity, aclpb.ACLChange_Admin) {
			err = fmt.Errorf("user %s cannot make changes", c.Content.Identity)
			return false
		}

		if c.Id == beforeId {
			foundId = true
			return false
		}

		return true
	}

	for {
		sb.tree.IterateSkip(sb.tree.root.Id, startChange.Id, iterFunc)
		if err == nil {
			break
		}

		// the user is forbidden to access the document
		if err == ErrDocumentForbidden {
			return nil, foundId, err
		}

		// otherwise we have to continue from the change which we had
		err = nil
	}

	return state, foundId, err
}
