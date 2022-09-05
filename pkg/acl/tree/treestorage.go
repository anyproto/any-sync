package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/gogo/protobuf/proto"
	"time"
)

func CreateNewTreeStorage(
	acc *account.AccountData,
	aclList list.ACLList,
	content proto.Marshaler,
	create storage.CreatorFunc) (storage.TreeStorage, error) {

	state := aclList.ACLState()
	change := &aclpb.Change{
		AclHeadId:          aclList.Head().Id,
		CurrentReadKeyHash: state.CurrentReadKeyHash(),
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           acc.Identity,
		IsSnapshot:         true,
	}

	marshalledData, err := content.Marshal()
	if err != nil {
		return nil, err
	}

	readKey, err := state.CurrentReadKey()
	if err != nil {
		return nil, err
	}

	encrypted, err := readKey.Encrypt(marshalledData)
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

	changeId, err := cid.NewCIDFromBytes(fullMarshalledChange)
	if err != nil {
		return nil, err
	}

	rawChange := &aclpb.RawChange{
		Payload:   fullMarshalledChange,
		Signature: signature,
		Id:        changeId,
	}
	header, treeId, err := createTreeHeaderAndId(rawChange, aclpb.Header_DocTree, aclList.ID())
	if err != nil {
		return nil, err
	}

	thr, err := create(treeId, header, []*aclpb.RawChange{rawChange})
	if err != nil {
		return nil, err
	}

	err = thr.SetHeads([]string{changeId})
	if err != nil {
		return nil, err
	}
	return thr, nil
}

func createTreeHeaderAndId(change *aclpb.RawChange, treeType aclpb.HeaderDocType, aclListId string) (*aclpb.Header, string, error) {
	header := &aclpb.Header{
		FirstId:   change.Id,
		DocType:   treeType,
		AclListId: aclListId,
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
