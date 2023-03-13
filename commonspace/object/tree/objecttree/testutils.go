package objecttree

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
)

type MockChangeCreator struct{}

func NewMockChangeCreator() *MockChangeCreator {
	return &MockChangeCreator{}
}

func (c *MockChangeCreator) CreateRoot(id, aclId string) *treechangeproto.RawTreeChangeWithId {
	aclChange := &treechangeproto.RootChange{
		AclHeadId: aclId,
	}
	res, _ := aclChange.Marshal()

	raw := &treechangeproto.RawTreeChange{
		Payload:   res,
		Signature: nil,
	}
	rawMarshalled, _ := raw.Marshal()

	return &treechangeproto.RawTreeChangeWithId{
		RawChange: rawMarshalled,
		Id:        id,
	}
}

func (c *MockChangeCreator) CreateRaw(id, aclId, snapshotId string, isSnapshot bool, prevIds ...string) *treechangeproto.RawTreeChangeWithId {
	aclChange := &treechangeproto.TreeChange{
		TreeHeadIds:    prevIds,
		AclHeadId:      aclId,
		SnapshotBaseId: snapshotId,
		ChangesData:    nil,
		IsSnapshot:     isSnapshot,
	}
	res, _ := aclChange.Marshal()

	raw := &treechangeproto.RawTreeChange{
		Payload:   res,
		Signature: nil,
	}
	rawMarshalled, _ := raw.Marshal()

	return &treechangeproto.RawTreeChangeWithId{
		RawChange: rawMarshalled,
		Id:        id,
	}
}

func (c *MockChangeCreator) CreateNewTreeStorage(treeId, aclHeadId string) treestorage.TreeStorage {
	root := c.CreateRoot(treeId, aclHeadId)
	treeStorage, _ := treestorage.NewInMemoryTreeStorage(root, []string{root.Id}, []*treechangeproto.RawTreeChangeWithId{root})
	return treeStorage
}

func BuildTestableTree(aclList list.AclList, treeStorage treestorage.TreeStorage) (ObjectTree, error) {
	root, _ := treeStorage.Root()
	changeBuilder := &nonVerifiableChangeBuilder{
		ChangeBuilder: NewChangeBuilder(nil, root),
	}
	deps := objectTreeDeps{
		changeBuilder:   changeBuilder,
		treeBuilder:     newTreeBuilder(treeStorage, changeBuilder),
		treeStorage:     treeStorage,
		rawChangeLoader: newRawChangeLoader(treeStorage, changeBuilder),
		validator:       &noOpTreeValidator{},
		aclList:         aclList,
	}

	return buildObjectTree(deps)
}
