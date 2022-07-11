package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
)

func BuildThreadWithACL(
	acc *account.AccountData,
	build func(builder ChangeBuilder),
	create func(change *thread.RawChange) (thread.Thread, error)) (thread.Thread, error) {
	bld := newChangeBuilder()
	bld.Init(
		newACLState(acc.Identity, acc.EncKey, keys.NewEd25519Decoder()),
		&Tree{},
		acc)
	build(bld)
	bld.SetMakeSnapshot(true)

	change, payload, err := bld.Build()
	if err != nil {
		return nil, err
	}

	rawChange := &thread.RawChange{
		Payload:   payload,
		Signature: change.Signature(),
		Id:        change.CID(),
	}
	return create(rawChange)
}
