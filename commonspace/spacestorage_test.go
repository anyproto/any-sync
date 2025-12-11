package commonspace

import (
	"context"
	"path/filepath"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/spacepayloads"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/util/crypto"
)

func newStorageCreatePayload(t *testing.T) spacestorage.SpaceStorageCreatePayload {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	metaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	readKey := crypto.NewAES()
	meta := []byte("account")
	payload := spacepayloads.SpaceCreatePayload{
		SigningKey:     keys.SignKey,
		SpaceType:      "space",
		ReplicationKey: 10,
		SpacePayload:   nil,
		MasterKey:      masterKey,
		ReadKey:        readKey,
		MetadataKey:    metaKey,
		Metadata:       meta,
	}
	createSpace, err := spacepayloads.StoragePayloadForSpaceCreate(payload)
	require.NoError(t, err)
	return createSpace
}

var ctx = context.Background()

func TestCreateSpaceStorageFailed_EmptyStorage(t *testing.T) {
	payload := newStorageCreatePayload(t)
	store, err := anystore.Open(ctx, filepath.Join(t.TempDir(), "store.db"), nil)
	require.NoError(t, err)
	payload.SpaceSettingsWithId.RawChange = nil
	_, err = spacestorage.Create(ctx, store, payload)
	require.Error(t, err)
	collNames, err := store.GetCollectionNames(ctx)
	require.NoError(t, err)
	require.Empty(t, collNames)
}

func TestCreate_ReturnsErrSpaceStorageExists_WhenStorageAlreadyExists(t *testing.T) {
	payload := newStorageCreatePayload(t)
	store, err := anystore.Open(ctx, filepath.Join(t.TempDir(), "store.db"), nil)
	require.NoError(t, err)
	defer store.Close()

	// First creation should succeed
	_, err = spacestorage.Create(ctx, store, payload)
	require.NoError(t, err)

	// Second creation with the same payload should return ErrSpaceStorageExists
	_, err = spacestorage.Create(ctx, store, payload)
	require.ErrorIs(t, err, spacestorage.ErrSpaceStorageExists)
}