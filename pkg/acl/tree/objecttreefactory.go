package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

type ObjectTreeCreatePayload struct {
	AccountData *account.AccountData
	HeaderData  []byte
	ChangeData  []byte
	TreeType    aclpb.TreeHeaderType
}

func BuildObjectTree(treeStorage storage.TreeStorage, listener ObjectTreeUpdateListener, aclList list.ACLList) (ObjectTree, error) {
	deps := defaultObjectTreeDeps(treeStorage, listener, aclList)
	return buildObjectTree(deps)
}

func CreateObjectTree(
	payload ObjectTreeCreatePayload,
	listener ObjectTreeUpdateListener,
	aclList list.ACLList,
	createStorage storage.TreeStorageCreatorFunc) (objTree ObjectTree, err error) {
	aclList.RLock()
	var (
		deps        = defaultObjectTreeDeps(nil, listener, aclList)
		state       = aclList.ACLState()
		aclId       = aclList.ID()
		aclHeadId   = aclList.Head().Id
		readKeyHash = state.CurrentReadKeyHash()
	)
	readKey, err := state.CurrentReadKey()
	aclList.RUnlock()

	if err != nil {
		return
	}

	// create first change
	cnt := BuilderContent{
		treeHeadIds:        nil,
		aclHeadId:          aclHeadId,
		snapshotBaseId:     "",
		currentReadKeyHash: readKeyHash,
		isSnapshot:         true,
		readKey:            readKey,
		identity:           payload.AccountData.Identity,
		signingKey:         payload.AccountData.SignKey,
		content:            payload.ChangeData,
	}

	_, raw, err := deps.changeBuilder.BuildContent(cnt)
	if err != nil {
		return
	}

	// create header
	header, id, err := createTreeHeaderAndId(
		raw,
		payload.TreeType,
		aclId,
		payload.AccountData.Identity,
		payload.HeaderData)
	if err != nil {
		return
	}

	// create storage
	st, err := createStorage(storage.TreeStorageCreatePayload{
		TreeId:  id,
		Header:  header,
		Changes: []*aclpb.RawTreeChangeWithId{raw},
		Heads:   []string{raw.Id},
	})
	if err != nil {
		return
	}

	deps.treeStorage = st
	return buildObjectTree(deps)
}

func buildObjectTree(deps objectTreeDeps) (ObjectTree, error) {
	objTree := &objectTree{
		treeStorage:     deps.treeStorage,
		updateListener:  deps.updateListener,
		treeBuilder:     deps.treeBuilder,
		validator:       deps.validator,
		aclList:         deps.aclList,
		changeBuilder:   deps.changeBuilder,
		rawChangeLoader: deps.rawChangeLoader,
		tree:            nil,
		keys:            make(map[uint64]*symmetric.Key),
		tmpChangesBuf:   make([]*Change, 0, 10),
		difSnapshotBuf:  make([]*aclpb.RawTreeChangeWithId, 0, 10),
		notSeenIdxBuf:   make([]int, 0, 10),
		newSnapshotsBuf: make([]*Change, 0, 10),
	}

	err := objTree.rebuildFromStorage(nil)
	if err != nil {
		return nil, err
	}
	storageHeads, err := objTree.treeStorage.Heads()
	if err != nil {
		return nil, err
	}

	// comparing rebuilt heads with heads in storage
	// in theory it can happen that we didn't set heads because the process has crashed
	// therefore we want to set them later
	if !slice.UnsortedEquals(storageHeads, objTree.tree.Heads()) {
		log.With(zap.Strings("storage", storageHeads), zap.Strings("rebuilt", objTree.tree.Heads())).
			Errorf("the heads in storage and objTree are different")
		err = objTree.treeStorage.SetHeads(objTree.tree.Heads())
		if err != nil {
			return nil, err
		}
	}

	objTree.id, err = objTree.treeStorage.ID()
	if err != nil {
		return nil, err
	}
	objTree.header, err = objTree.treeStorage.Header()
	if err != nil {
		return nil, err
	}

	return objTree, nil
}

func createTreeHeaderAndId(
	raw *aclpb.RawTreeChangeWithId,
	treeType aclpb.TreeHeaderType,
	aclId string,
	identity []byte,
	headerData []byte) (header *aclpb.TreeHeader, treeId string, err error) {
	header = &aclpb.TreeHeader{
		FirstId:        raw.Id,
		TreeHeaderType: treeType,
		AclId:          aclId,
		Identity:       identity,
		Data:           headerData,
	}
	marshalledHeader, err := proto.Marshal(header)
	if err != nil {
		return
	}

	treeId, err = cid.NewCIDFromBytes(marshalledHeader)
	return
}
