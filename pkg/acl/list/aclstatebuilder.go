package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
)

type aclStateBuilder struct {
	signPrivKey signingkey.PrivKey
	encPrivKey  encryptionkey.PrivKey
	decoder     keys.Decoder
}

func newACLStateBuilderWithIdentity(decoder keys.Decoder, accountData *account.AccountData) *aclStateBuilder {
	return &aclStateBuilder{
		decoder:     decoder,
		signPrivKey: accountData.SignKey,
		encPrivKey:  accountData.EncKey,
	}
}

func newACLStateBuilder(decoder keys.Decoder) *aclStateBuilder {
	return &aclStateBuilder{
		decoder: decoder,
	}
}

func (sb *aclStateBuilder) Build(records []*ACLRecord) (state *ACLState, err error) {
	if sb.encPrivKey != nil && sb.signPrivKey != nil {
		state, err = newACLStateWithKeys(sb.signPrivKey, sb.encPrivKey)
		if err != nil {
			return
		}
	} else {
		state = newACLState(sb.decoder)
	}
	for _, rec := range records {
		err = state.applyRecord(rec)
		if err != nil {
			return nil, err
		}
	}

	return state, err
}
