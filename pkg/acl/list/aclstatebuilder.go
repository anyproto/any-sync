package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

type aclStateBuilder struct {
	signPrivKey signingkey.PrivKey
	encPrivKey  encryptionkey.PrivKey
	id          string
}

func newACLStateBuilderWithIdentity(accountData *account.AccountData) *aclStateBuilder {
	return &aclStateBuilder{
		signPrivKey: accountData.SignKey,
		encPrivKey:  accountData.EncKey,
	}
}

func newACLStateBuilder() *aclStateBuilder {
	return &aclStateBuilder{}
}

func (sb *aclStateBuilder) Init(id string) {
	sb.id = id
}

func (sb *aclStateBuilder) Build(records []*ACLRecord) (state *ACLState, err error) {
	if sb.encPrivKey != nil && sb.signPrivKey != nil {
		state, err = newACLStateWithKeys(sb.id, sb.signPrivKey, sb.encPrivKey)
		if err != nil {
			return
		}
	} else {
		state = newACLState(sb.id)
	}
	for _, rec := range records {
		err = state.applyRecord(rec)
		if err != nil {
			return nil, err
		}
	}

	return state, err
}
