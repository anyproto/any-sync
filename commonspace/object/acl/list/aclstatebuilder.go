package list

import (
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
)

type aclStateBuilder struct {
	signPrivKey signingkey.PrivKey
	encPrivKey  encryptionkey.PrivKey
	id          string
}

func newAclStateBuilderWithIdentity(accountData *accountdata.AccountData) *aclStateBuilder {
	return &aclStateBuilder{
		signPrivKey: accountData.SignKey,
		encPrivKey:  accountData.EncKey,
	}
}

func newAclStateBuilder() *aclStateBuilder {
	return &aclStateBuilder{}
}

func (sb *aclStateBuilder) Init(id string) {
	sb.id = id
}

func (sb *aclStateBuilder) Build(records []*AclRecord) (state *AclState, err error) {
	if sb.encPrivKey != nil && sb.signPrivKey != nil {
		state, err = newAclStateWithKeys(sb.id, sb.signPrivKey, sb.encPrivKey)
		if err != nil {
			return
		}
	} else {
		state = newAclState(sb.id)
	}
	for _, rec := range records {
		err = state.applyRecord(rec)
		if err != nil {
			return nil, err
		}
	}

	return state, err
}

func (sb *aclStateBuilder) Append(state *AclState, records []*AclRecord) (err error) {
	for _, rec := range records {
		err = state.applyRecord(rec)
		if err != nil {
			return
		}
	}
	return
}
