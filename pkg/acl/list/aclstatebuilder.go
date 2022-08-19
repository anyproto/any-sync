package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
)

type aclStateBuilder struct {
	identity string
	key      encryptionkey.PrivKey
	decoder  keys.Decoder
}

func newACLStateBuilderWithIdentity(decoder keys.Decoder, accountData *account.AccountData) *aclStateBuilder {
	return &aclStateBuilder{
		decoder:  decoder,
		identity: accountData.Identity,
		key:      accountData.EncKey,
	}
}

func newACLStateBuilder() *aclStateBuilder {
	return &aclStateBuilder{}
}

func (sb *aclStateBuilder) Build(records []*Record) (*ACLState, error) {
	var (
		err   error
		state *ACLState
	)

	if sb.decoder != nil {
		state = newACLStateWithIdentity(sb.identity, sb.key, sb.decoder)
	} else {
		state = newACLState()
	}
	for _, rec := range records {
		err = state.applyChangeAndUpdate(rec)
		if err != nil {
			return nil, err
		}
	}

	return state, err
}
