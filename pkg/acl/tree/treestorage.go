package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/gogo/protobuf/proto"
	"time"
)

func CreateNewTreeStorageWithACL(
	acc *account.AccountData,
	build func(builder ACLChangeBuilder) error,
	create treestorage.CreatorFunc) (treestorage.TreeStorage, error) {
	bld := newACLChangeBuilder()
	bld.Init(
		newACLStateWithIdentity(acc.Identity, acc.EncKey, signingkey.NewEd25519PubKeyDecoder()),
		&Tree{},
		acc)
	err := build(bld)
	if err != nil {
		return nil, err
	}

	change, payload, err := bld.BuildAndApply()
	if err != nil {
		return nil, err
	}

	rawChange := &aclpb.RawChange{
		Payload:   payload,
		Signature: change.Signature(),
		Id:        change.CID(),
	}
	header, id, err := createTreeHeaderAndId(rawChange, treepb.TreeHeader_ACLTree)
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

func CreateNewTreeStorage(
	acc *account.AccountData,
	aclTree ACLTree,
	content proto.Marshaler,
	create treestorage.CreatorFunc) (treestorage.TreeStorage, error) {

	state := aclTree.ACLState()
	change := &aclpb.Change{
		AclHeadIds:         aclTree.Heads(),
		CurrentReadKeyHash: state.currentReadKeyHash,
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           acc.Identity,
	}

	marshalledData, err := content.Marshal()
	if err != nil {
		return nil, err
	}
	encrypted, err := state.userReadKeys[state.currentReadKeyHash].Encrypt(marshalledData)
	if err != nil {
		return nil, err
	}
	change.ChangesData = encrypted

	fullMarshalledChange, err := proto.Marshal(change)
	if err != nil {
		return nil, err
	}
	signature, err := acc.SignKey.Sign(fullMarshalledChange)
	if err != nil {
		return nil, err
	}
	id, err := cid.NewCIDFromBytes(fullMarshalledChange)
	if err != nil {
		return nil, err
	}

	rawChange := &aclpb.RawChange{
		Payload:   fullMarshalledChange,
		Signature: signature,
		Id:        id,
	}
	header, id, err := createTreeHeaderAndId(rawChange, treepb.TreeHeader_DocTree)
	if err != nil {
		return nil, err
	}

	thr, err := create(id, header, []*aclpb.RawChange{rawChange})
	if err != nil {
		return nil, err
	}

	err = thr.SetHeads([]string{id})
	if err != nil {
		return nil, err
	}
	return thr, nil
}

func createTreeHeaderAndId(change *aclpb.RawChange, treeType treepb.TreeHeaderTreeType) (*treepb.TreeHeader, string, error) {
	header := &treepb.TreeHeader{
		FirstChangeId: change.Id,
		Type:          treeType,
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
