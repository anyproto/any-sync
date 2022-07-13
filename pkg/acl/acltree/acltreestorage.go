package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
)

func BuildTreeStorageWithACL(
	acc *account.AccountData,
	build func(builder ChangeBuilder) error,
	create func(change *treestorage.RawChange) (treestorage.TreeStorage, error)) (treestorage.TreeStorage, error) {
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

	rawChange := &treestorage.RawChange{
		Payload:   payload,
		Signature: change.Signature(),
		Id:        change.CID(),
	}

	thr, err := create(rawChange)
	if err != nil {
		return nil, err
	}

	err = thr.SetHeads([]string{change.CID()})
	if err != nil {
		return nil, err
	}
	return thr, nil
}
