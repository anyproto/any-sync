package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/thread"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
)

func BuildThreadWithACL(
	acc *account.AccountData,
	build func(builder ChangeBuilder) error,
	create func(change *thread.RawChange) (thread.Thread, error)) (thread.Thread, error) {
	bld := newChangeBuilder()
	bld.Init(
		newACLState(acc.Identity, acc.EncKey, keys.NewEd25519Decoder()),
		&Tree{},
		acc)
	err := build(bld)
	if err != nil {
		return nil, err
	}
	bld.SetMakeSnapshot(true)

	change, payload, err := bld.BuildAndApply()
	if err != nil {
		return nil, err
	}

	rawChange := &thread.RawChange{
		Payload:   payload,
		Signature: change.Signature(),
		Id:        change.CID(),
	}

	thr, err := create(rawChange)
	if err != nil {
		return nil, err
	}

	thr.SetHeads([]string{change.CID()})
	return thr, nil
}
