package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	storage2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
	"go.uber.org/zap"
	"math/rand"
	"time"
)

type ObjectTreeCreatePayload struct {
	SignKey     signingkey.PrivKey
	ChangeType  string
	SpaceId     string
	Identity    []byte
	IsEncrypted bool
}

func BuildObjectTree(treeStorage storage2.TreeStorage, aclList list.ACLList) (ObjectTree, error) {
	rootChange, err := treeStorage.Root()
	if err != nil {
		return nil, err
	}
	deps := defaultObjectTreeDeps(rootChange, treeStorage, aclList)
	return buildObjectTree(deps)
}

func CreateDerivedObjectTree(
	payload ObjectTreeCreatePayload,
	aclList list.ACLList,
	createStorage storage2.TreeStorageCreatorFunc) (objTree ObjectTree, err error) {
	return createObjectTree(payload, 0, nil, aclList, createStorage)
}

func CreateObjectTree(
	payload ObjectTreeCreatePayload,
	aclList list.ACLList,
	createStorage storage2.TreeStorageCreatorFunc) (objTree ObjectTree, err error) {
	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)
	if err != nil {
		return
	}
	return createObjectTree(payload, time.Now().UnixNano(), bytes, aclList, createStorage)
}

func createObjectTree(
	payload ObjectTreeCreatePayload,
	timestamp int64,
	seed []byte,
	aclList list.ACLList,
	createStorage storage2.TreeStorageCreatorFunc) (objTree ObjectTree, err error) {
	aclList.RLock()
	aclHeadId := aclList.Head().Id
	aclList.RUnlock()

	if err != nil {
		return
	}
	cnt := InitialContent{
		AclHeadId:  aclHeadId,
		Identity:   payload.Identity,
		SigningKey: payload.SignKey,
		SpaceId:    payload.SpaceId,
		ChangeType: payload.ChangeType,
		Timestamp:  timestamp,
		Seed:       seed,
	}

	_, raw, err := NewChangeBuilder(common.NewKeychain(), nil).BuildInitialContent(cnt)
	if err != nil {
		return
	}

	// create storage
	st, err := createStorage(storage2.TreeStorageCreatePayload{
		TreeId:        raw.Id,
		RootRawChange: raw,
		Changes:       []*treechangeproto.RawTreeChangeWithId{raw},
		Heads:         []string{raw.Id},
	})
	if err != nil {
		return
	}

	return BuildObjectTree(st, aclList)
}

func buildObjectTree(deps objectTreeDeps) (ObjectTree, error) {
	objTree := &objectTree{
		treeStorage:     deps.treeStorage,
		treeBuilder:     deps.treeBuilder,
		validator:       deps.validator,
		aclList:         deps.aclList,
		changeBuilder:   deps.changeBuilder,
		rawChangeLoader: deps.rawChangeLoader,
		tree:            nil,
		keys:            make(map[uint64]*symmetric.Key),
		newChangesBuf:   make([]*Change, 0, 10),
		difSnapshotBuf:  make([]*treechangeproto.RawTreeChangeWithId, 0, 10),
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

	objTree.id = objTree.treeStorage.Id()

	objTree.root, err = objTree.treeStorage.Root()
	if err != nil {
		return nil, err
	}

	// verifying root
	_, err = objTree.changeBuilder.ConvertFromRaw(objTree.root, true)
	if err != nil {
		return nil, err
	}

	return objTree, nil
}
