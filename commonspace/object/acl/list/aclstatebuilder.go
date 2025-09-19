package list

import (
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/util/crypto"
)

type aclStateBuilder struct {
	privKey crypto.PrivKey
	id      string
}

func newAclStateBuilderWithIdentity(keys *accountdata.AccountKeys) *aclStateBuilder {
	return &aclStateBuilder{
		privKey: keys.SignKey,
	}
}

func newAclStateBuilder() *aclStateBuilder {
	return &aclStateBuilder{}
}

func (sb *aclStateBuilder) Init(id string) {
	sb.id = id
}

func (sb *aclStateBuilder) Build(records []*AclRecord, list *aclList) (state *AclState, err error) {
	if len(records) == 0 {
		return nil, ErrIncorrectRecordSequence
	}
	if sb.privKey != nil {
		// here it builds acl state with sb.privKey, which is usually account key.
		// it uses it to decrypt read key -> metadata key in saveKeysFromRoot
		// we probably shoud go different way with onetoone, and use sharedKey as st.key
		state, err = newAclStateWithKeys(records[0], sb.privKey, list.verifier)
		if err != nil {
			return
		}
	} else {
		state, err = newAclState(records[0], list.verifier)
		if err != nil {
			return
		}
	}
	state.list = list
	for _, rec := range records[1:] {
		err = state.ApplyRecord(rec)
		if err != nil {
			return nil, err
		}
	}

	return state, err
}

func (sb *aclStateBuilder) Append(state *AclState, records []*AclRecord) (err error) {
	for _, rec := range records {
		err = state.ApplyRecord(rec)
		if err != nil {
			return
		}
	}
	return
}
