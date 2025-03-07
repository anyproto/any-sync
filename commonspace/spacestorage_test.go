package commonspace

import (
	"context"
	"path/filepath"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
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
	payload := SpaceCreatePayload{
		SigningKey:     keys.SignKey,
		SpaceType:      "space",
		ReplicationKey: 10,
		SpacePayload:   nil,
		MasterKey:      masterKey,
		ReadKey:        readKey,
		MetadataKey:    metaKey,
		Metadata:       meta,
	}
	createSpace, err := StoragePayloadForSpaceCreate(payload)
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