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
	create storage.TreeStorageCreatorFunc) (thr storage.TreeStorage, err error) {

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
		return
	}

	readKey, err := state.CurrentReadKey()
	if err != nil {
		return
	}

	encrypted, err := readKey.Encrypt(marshalledData)
	if err != nil {
		return
	}

	change.ChangesData = encrypted

	fullMarshalledChange, err := proto.Marshal(change)
	if err != nil {
		return
	}

	signature, err := acc.SignKey.Sign(fullMarshalledChange)
	if err != nil {
		return
	}

	changeId, err := cid.NewCIDFromBytes(fullMarshalledChange)
	if err != nil {
		return
	}

	rawChange := &aclpb.RawChange{
		Payload:   fullMarshalledChange,
		Signature: signature,
		Id:        changeId,
	}
	header, treeId, err := createTreeHeaderAndId(rawChange, aclpb.Header_DocTree, aclList.ID())
	if err != nil {
		return
	}

	return create(storage.TreeStorageCreatePayload{
		TreeId:  treeId,
		Header:  header,
		Changes: []*aclpb.RawChange{rawChange},
		Heads:   []string{rawChange.Id},
	})
}

func createTreeHeaderAndId(change *aclpb.RawChange, treeType aclpb.HeaderDocType, aclListId string) (header *aclpb.Header, treeId string, err error) {
	header = &aclpb.Header{
		FirstId:   change.Id,
		DocType:   treeType,
		AclListId: aclListId,
	}
	marshalledHeader, err := proto.Marshal(header)
	if err != nil {
		return
	}

	treeId, err = cid.NewCIDFromBytes(marshalledHeader)
	return
}
