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

func (sb *aclStateBuilder) Build(records []*AclRecord) (state *AclState, err error) {
	if len(records) == 0 {
		return nil, ErrIncorrectRecordSequence
	}
	if sb.privKey != nil {
		state, err = newAclStateWithKeys(records[0], sb.privKey)
		if err != nil {
			return
		}
	} else {
		state, err = newAclState(records[0])
		if err != nil {
			return
		}
	}
	for _, rec := range records[1:] {
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
