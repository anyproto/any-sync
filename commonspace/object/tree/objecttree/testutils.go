package objecttree

import (
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
	"github.com/anytypeio/any-sync/util/crypto"
	libcrypto "github.com/libp2p/go-libp2p/core/crypto"
)

type mockPubKey struct {
}

const mockKeyValue = "mockKey"

func (m mockPubKey) Equals(key crypto.Key) bool {
	return true
}

func (m mockPubKey) Raw() ([]byte, error) {
	return []byte(mockKeyValue), nil
}

func (m mockPubKey) Encrypt(message []byte) ([]byte, error) {
	return message, nil
}

func (m mockPubKey) Verify(data []byte, sig []byte) (bool, error) {
	return true, nil
}

func (m mockPubKey) Marshall() ([]byte, error) {
	return []byte(mockKeyValue), nil
}

func (m mockPubKey) Storage() []byte {
	return []byte(mockKeyValue)
}

func (m mockPubKey) Account() string {
	return mockKeyValue
}

func (m mockPubKey) Network() string {
	return mockKeyValue
}

func (m mockPubKey) PeerId() string {
	return mockKeyValue
}

func (m mockPubKey) LibP2P() (libcrypto.PubKey, error) {
	return nil, fmt.Errorf("can't be converted in libp2p")
}

type mockKeyStorage struct {
}

func newMockKeyStorage() mockKeyStorage {
	return mockKeyStorage{}
}

func (m mockKeyStorage) PubKeyFromProto(protoBytes []byte) (crypto.PubKey, error) {
	return mockPubKey{}, nil
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
