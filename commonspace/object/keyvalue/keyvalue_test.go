package keyvalue

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/spacepayloads"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/util/crypto"
)

func TestKeyValueService(t *testing.T) {
	t.Run("different keys", func(t *testing.T) {
		fxClient, fxServer, serverPeer := prepareFixtures(t)
		fxClient.add(t, "key1", []byte("value1"))
		fxClient.add(t, "key2", []byte("value2"))
		fxServer.add(t, "key3", []byte("value3"))
		fxServer.add(t, "key4", []byte("value4"))
		err := fxClient.SyncWithPeer(serverPeer)
		require.NoError(t, err)
		fxClient.limiter.Close()
		fxClient.check(t, "key3", []byte("value3"))
		fxClient.check(t, "key4", []byte("value4"))
		fxServer.check(t, "key1", []byte("value1"))
		fxServer.check(t, "key2", []byte("value2"))
	})

	t.Run("change same keys, different values", func(t *testing.T) {
		fxClient, fxServer, serverPeer := prepareFixtures(t)
		fxClient.add(t, "key1", []byte("value1"))
		fxServer.add(t, "key1", []byte("value2"))
		err := fxClient.SyncWithPeer(serverPeer)
		require.NoError(t, err)
		fxClient.limiter.Close()
		fxClient.check(t, "key1", []byte("value1"))
		fxClient.check(t, "key1", []byte("value2"))
		fxServer.check(t, "key1", []byte("value1"))
		fxServer.check(t, "key1", []byte("value2"))
		fxClient.add(t, "key1", []byte("value1-2"))
		fxServer.add(t, "key1", []byte("value2-2"))
		err = fxClient.SyncWithPeer(serverPeer)
		require.NoError(t, err)
		fxClient.limiter.Close()
		fxClient.check(t, "key1", []byte("value1-2"))
		fxClient.check(t, "key1", []byte("value2-2"))
		fxServer.check(t, "key1", []byte("value1-2"))
		fxServer.check(t, "key1", []byte("value2-2"))
	})

	t.Run("random keys and values", func(t *testing.T) {
		rand.Seed(time.Now().UnixNano())
		diffEntries := 100
		ovelappingEntries := 10
		fxClient, fxServer, serverPeer := prepareFixtures(t)
		numClientEntries := 5 + rand.Intn(diffEntries)
		numServerEntries := 5 + rand.Intn(diffEntries)
		allKeys := make(map[string]bool)
		for i := 0; i < numClientEntries; i++ {
			key := fmt.Sprintf("client-key-%d", i)
			value := []byte(fmt.Sprintf("client-value-%d", i))
			fxClient.add(t, key, value)
			allKeys[key] = true
		}
		for i := 0; i < numServerEntries; i++ {
			key := fmt.Sprintf("server-key-%d", i)
			value := []byte(fmt.Sprintf("server-value-%d", i))
			fxServer.add(t, key, value)
			allKeys[key] = true
		}
		numOverlappingKeys := 3 + rand.Intn(ovelappingEntries)
		for i := 0; i < numOverlappingKeys; i++ {
			key := fmt.Sprintf("overlap-key-%d", i)
			clientValue := []byte(fmt.Sprintf("client-overlap-value-%d", i))
			serverValue := []byte(fmt.Sprintf("server-overlap-value-%d", i))
			fxClient.add(t, key, clientValue)
			fxServer.add(t, key, serverValue)
			allKeys[key] = true
		}
		err := fxClient.SyncWithPeer(serverPeer)
		require.NoError(t, err)
		fxClient.limiter.Close()

		for key := range allKeys {
			if strings.HasPrefix(key, "client-key-") {
				i, _ := strconv.Atoi(strings.TrimPrefix(key, "client-key-"))
				value := []byte(fmt.Sprintf("client-value-%d", i))
				fxClient.check(t, key, value)
				fxServer.check(t, key, value)
			}
			if strings.HasPrefix(key, "server-key-") {
				i, _ := strconv.Atoi(strings.TrimPrefix(key, "server-key-"))
				value := []byte(fmt.Sprintf("server-value-%d", i))
				fxClient.check(t, key, value)
				fxServer.check(t, key, value)
			}
		}
		for i := 0; i < numOverlappingKeys; i++ {
			key := fmt.Sprintf("overlap-key-%d", i)
			clientValue := []byte(fmt.Sprintf("client-overlap-value-%d", i))
			serverValue := []byte(fmt.Sprintf("server-overlap-value-%d", i))

			fxClient.check(t, key, clientValue)
			fxClient.check(t, key, serverValue)
			fxServer.check(t, key, clientValue)
			fxServer.check(t, key, serverValue)
		}
	})
}

func prepareFixtures(t *testing.T) (fxClient *fixture, fxServer *fixture, serverPeer peer.Peer) {
	firstKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	secondKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	secondKeys.SignKey = firstKeys.SignKey
	payload := newStorageCreatePayload(t, firstKeys)
	fxClient = newFixture(t, firstKeys, payload)
	fxServer = newFixture(t, secondKeys, payload)
	serverConn, clientConn := rpctest.MultiConnPair(firstKeys.PeerId, secondKeys.PeerId)
	serverPeer, err = peer.NewPeer(serverConn, fxClient.server)
	require.NoError(t, err)
	_, err = peer.NewPeer(clientConn, fxServer.server)
	require.NoError(t, err)
	return
}

var ctx = context.Background()

type noOpSyncClient struct{}

func (n noOpSyncClient) Broadcast(ctx context.Context, objectId string, keyValues ...innerstorage.KeyValue) error {
	return nil
}

type fixture struct {
	*keyValueService
	server *rpctest.TestServer
}

func newFixture(t *testing.T, keys *accountdata.AccountKeys, spacePayload spacestorage.SpaceStorageCreatePayload) *fixture {
	storePath := filepath.Join(t.TempDir(), "store.db")
	anyStore, err := anystore.Open(ctx, storePath, nil)
	require.NoError(t, err)
	storage, err := spacestorage.Create(ctx, anyStore, spacePayload)
	require.NoError(t, err)
	aclStorage, err := storage.AclStorage()
	require.NoError(t, err)
	aclList, err := list.BuildAclListWithIdentity(keys, aclStorage, list.NoOpAcceptorVerifier{})
	require.NoError(t, err)
	storageId := "kv.storage"
	rpcHandler := rpctest.NewTestServer()
	defaultStorage, err := keyvaluestorage.New(ctx,
		storageId,
		anyStore,
		storage.HeadStorage(),
		keys,
		noOpSyncClient{},
		aclList,
		keyvaluestorage.NoOpIndexer{})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(ctx)
	service := &keyValueService{
		spaceId:       storage.Id(),
		storageId:     storageId,
		limiter:       newConcurrentLimiter(),
		ctx:           ctx,
		cancel:        cancel,
		clientFactory: spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceSyncClient),
		defaultStore:  defaultStorage,
	}
	require.NoError(t, spacesyncproto.DRPCRegisterSpaceSync(rpcHandler, &testServer{service: service, t: t}))
	return &fixture{
		keyValueService: service,
		server:          rpcHandler,
	}
}

func (fx *fixture) add(t *testing.T, key string, value []byte) {
	err := fx.defaultStore.Set(ctx, key, value)
	require.NoError(t, err)
}

func (fx *fixture) check(t *testing.T, key string, value []byte) (isFound bool) {
	err := fx.defaultStore.GetAll(ctx, key, func(decryptor keyvaluestorage.Decryptor, values []innerstorage.KeyValue) error {
		for _, v := range values {
			decryptedValue, err := decryptor(v)
			require.NoError(t, err)
			if bytes.Equal(value, decryptedValue) {
				isFound = true
				break
			}
		}
		return nil
	})
	require.NoError(t, err)
	return
}

func newStorageCreatePayload(t *testing.T, keys *accountdata.AccountKeys) spacestorage.SpaceStorageCreatePayload {
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

type testServer struct {
	spacesyncproto.DRPCSpaceSyncUnimplementedServer
	service *keyValueService
	t       *testing.T
}

func (t *testServer) StoreDiff(ctx context.Context, req *spacesyncproto.StoreDiffRequest) (*spacesyncproto.StoreDiffResponse, error) {
	return t.service.HandleStoreDiffRequest(ctx, req)
}

func (t *testServer) StoreElements(stream spacesyncproto.DRPCSpaceSync_StoreElementsStream) error {
	msg, err := stream.Recv()
	require.NoError(t.t, err)
	require.NotEmpty(t.t, msg.SpaceId)
	return t.service.HandleStoreElementsRequest(ctx, stream)
}
