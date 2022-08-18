package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
)

type aclStateBuilder struct {
	log      ACLList
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

func (sb *aclStateBuilder) Init(aclLog ACLList) error {
	sb.log = aclLog
	return nil
}

func (sb *aclStateBuilder) Build() (*ACLState, error) {
	var (
		err   error
		state *ACLState
	)

	if sb.decoder != nil {
		state = newACLStateWithIdentity(sb.identity, sb.key, sb.decoder)
	} else {
		state = newACLState()
	}

	sb.log.Iterate(func(c *Record) (isContinue bool) {
		err = state.applyChangeAndUpdate(c)
		return err == nil
	})

	return state, err
}
