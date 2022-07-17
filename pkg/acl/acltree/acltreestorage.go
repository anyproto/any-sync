package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/gogo/protobuf/proto"
)

func CreateNewTreeStorageWithACL(
	acc *account.AccountData,
	build func(builder ChangeBuilder) error,
	create treestorage.CreatorFunc) (treestorage.TreeStorage, error) {
	bld := newChangeBuilder()
	bld.Init(
		newACLState(acc.Identity, acc.EncKey, signingkey.NewEd25519PubKeyDecoder()),
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

	rawChange := &aclpb.RawChange{
		Payload:   payload,
		Signature: change.Signature(),
		Id:        change.CID(),
	}
	header, id, err := createTreeHeaderAndId(rawChange)
	if err != nil {
		return nil, err
	}

	thr, err := create(id, header, []*aclpb.RawChange{rawChange})
	if err != nil {
		return nil, err
	}

	err = thr.SetHeads([]string{change.CID()})
	if err != nil {
		return nil, err
	}
	return thr, nil
}

func createTreeHeaderAndId(change *aclpb.RawChange) (*treepb.TreeHeader, string, error) {
	header := &treepb.TreeHeader{
		FirstChangeId: change.Id,
		IsWorkspace:   false,
	}
	marshalledHeader, err := proto.Marshal(header)
	if err != nil {
		return nil, "", err
	}
	treeId, err := cid.NewCIDFromBytes(marshalledHeader)
	if err != nil {
		return nil, "", err
	}

	return header, treeId, nil
}
