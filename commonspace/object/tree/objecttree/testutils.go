package objecttree

import (
	"crypto/rand"
	"github.com/anytypeio/any-sync/commonspace/object/acl/list"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/util/crypto"
)

type mockKeyStorage struct {
	key crypto.PubKey
}

func newKeyStorage() mockKeyStorage {
	_, pk, _ := crypto.GenerateEd25519Key(rand.Reader)
	return mockKeyStorage{pk}
}

func (m mockKeyStorage) PubKeyFromProto(protoBytes []byte) (crypto.PubKey, error) {
	return m.key, nil
}

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
		ChangeBuilder: NewChangeBuilder(newKeyStorage(), root),
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
