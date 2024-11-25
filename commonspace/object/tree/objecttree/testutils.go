package objecttree

import (
	"context"
	"fmt"
	"testing"

	anystore "github.com/anyproto/any-store"
	libcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type mockPubKey struct {
}

const (
	mockKeyValue = "mockKey"
	mockDataType = "mockDataType"
)

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

type MockChangeCreator struct {
	store anystore.DB
}

type testStorage struct {
	Storage
	errAdd error
}

func (t testStorage) AddAll(ctx context.Context, changes []StorageChange, heads []string, commonSnapshot string) error {
	if t.errAdd != nil {
		return t.errAdd
	}
	return t.Storage.AddAll(ctx, changes, heads, commonSnapshot)
}

func NewMockChangeCreator(store anystore.DB) *MockChangeCreator {
	return &MockChangeCreator{
		store: store,
	}
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

func (c *MockChangeCreator) CreateDerivedRoot(id string, isDerived bool) *treechangeproto.RawTreeChangeWithId {
	aclChange := &treechangeproto.RootChange{
		IsDerived: isDerived,
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
	return c.CreateRawWithData(id, aclId, snapshotId, isSnapshot, nil, prevIds...)
}

func (c *MockChangeCreator) CreateRawWithData(id, aclId, snapshotId string, isSnapshot bool, data []byte, prevIds ...string) *treechangeproto.RawTreeChangeWithId {
	aclChange := &treechangeproto.TreeChange{
		TreeHeadIds:    prevIds,
		AclHeadId:      aclId,
		SnapshotBaseId: snapshotId,
		ChangesData:    data,
		IsSnapshot:     isSnapshot,
		DataType:       mockDataType,
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

func (c *MockChangeCreator) CreateNewTreeStorage(t *testing.T, treeId, aclHeadId string, isDerived bool) Storage {
	root := c.CreateRoot(treeId, aclHeadId)
	storageChangeBuilder = func(keys crypto.KeyStorage, rootChange *treechangeproto.RawTreeChangeWithId) ChangeBuilder {
		return &nonVerifiableChangeBuilder{
			ChangeBuilder: NewChangeBuilder(newMockKeyStorage(), rootChange),
		}
	}
	storage, err := createStorage(context.Background(), root, c.store)
	require.NoError(t, err)
	return testStorage{
		Storage: storage,
	}
}
