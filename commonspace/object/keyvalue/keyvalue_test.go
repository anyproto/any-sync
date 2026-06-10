package keyvalue

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
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
		foundClientKeys := make(map[string]bool)
		foundServerKeys := make(map[string]bool)
		err = fxClient.defaultStore.Iterate(context.Background(), func(decryptor keyvaluestorage.Decryptor, key string, values []innerstorage.KeyValue) (bool, error) {
			foundClientKeys[key] = true
			return true, nil
		})
		require.NoError(t, err)
		err = fxServer.defaultStore.Iterate(context.Background(), func(decryptor keyvaluestorage.Decryptor, key string, values []innerstorage.KeyValue) (bool, error) {
			foundServerKeys[key] = true
			return true, nil
		})
		require.NoError(t, err)
		require.True(t, mapEqual(allKeys, foundServerKeys), "expected all client keys to be found")
		require.True(t, mapEqual(foundClientKeys, foundServerKeys), "expected all client keys to be found")
	})
}

// TestSetRawSkipsInvalidValues asserts a value that fails to decode or verify
// poisons only itself: the rest of the batch still applies and the call
// succeeds. Otherwise one bad element wedges every subsequent pull at the same
// deterministic point, since the stream order is stable.
func TestSetRawSkipsInvalidValues(t *testing.T) {
	fxClient, fxServer, _ := prepareFixtures(t)
	for _, k := range []string{"good1", "poisoned", "good2"} {
		fxClient.add(t, k, []byte("v-"+k))
	}
	var protos []*spacesyncproto.StoreKeyValue
	err := fxClient.defaultStore.InnerStorage().IterateValues(ctx, func(kv innerstorage.KeyValue) (bool, error) {
		p := kv.Proto()
		p.Value = append([]byte(nil), p.Value...)
		p.PeerSignature = append([]byte(nil), p.PeerSignature...)
		p.IdentitySignature = append([]byte(nil), p.IdentitySignature...)
		if kv.Key == "poisoned" {
			p.IdentitySignature[0] ^= 0xff
		}
		protos = append(protos, p)
		return true, nil
	})
	require.NoError(t, err)
	require.Len(t, protos, 3)

	require.NoError(t, fxServer.defaultStore.SetRaw(ctx, protos...))
	require.True(t, fxServer.check(t, "good1", []byte("v-good1")), "valid value before the poisoned one must apply")
	require.True(t, fxServer.check(t, "good2", []byte("v-good2")), "valid value after the poisoned one must apply")
	require.False(t, fxServer.check(t, "poisoned", []byte("v-poisoned")), "the poisoned value itself must be skipped")
}

func TestKeyValueServiceIterate(t *testing.T) {
	t.Run("empty storage", func(t *testing.T) {
		fxClient, _, _ := prepareFixtures(t)
		var keys []string
		err := fxClient.defaultStore.Iterate(context.Background(), func(decryptor keyvaluestorage.Decryptor, key string, values []innerstorage.KeyValue) (bool, error) {
			keys = append(keys, key)
			return true, nil
		})
		require.NoError(t, err)
		require.Empty(t, keys, "expected no keys in empty storage")
	})

	t.Run("single key later value", func(t *testing.T) {
		fxClient, _, _ := prepareFixtures(t)
		err := fxClient.defaultStore.Set(context.Background(), "test-key", []byte("value1"))
		require.NoError(t, err)
		err = fxClient.defaultStore.Set(context.Background(), "test-key", []byte("value2"))
		require.NoError(t, err)
		var keys []string
		valueCount := 0
		err = fxClient.defaultStore.Iterate(context.Background(), func(decryptor keyvaluestorage.Decryptor, key string, values []innerstorage.KeyValue) (bool, error) {
			keys = append(keys, key)
			valueCount = len(values)

			for _, kv := range values {
				val, err := decryptor(kv)
				require.NoError(t, err)
				require.Equal(t, "value2", string(val))
			}
			return true, nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(keys), "expected one key")
		require.Equal(t, "test-key", keys[0], "expected key to be 'test-key'")
		require.Equal(t, 1, valueCount, "expected one value for key")
	})

	t.Run("multiple keys", func(t *testing.T) {
		fxClient, _, _ := prepareFixtures(t)
		testKeys := []string{"key1", "key2", "key3"}
		for _, key := range testKeys {
			err := fxClient.defaultStore.Set(context.Background(), key, []byte("value-"+key))
			require.NoError(t, err)
		}
		var foundKeys []string
		err := fxClient.defaultStore.Iterate(context.Background(), func(decryptor keyvaluestorage.Decryptor, key string, values []innerstorage.KeyValue) (bool, error) {
			foundKeys = append(foundKeys, key)
			require.Equal(t, 1, len(values), "Expected one value for key: "+key)
			val, err := decryptor(values[0])
			require.NoError(t, err)
			require.Equal(t, "value-"+key, string(val), "Value doesn't match for key: "+key)

			return true, nil
		})
		require.NoError(t, err)
		sort.Strings(foundKeys)
		sort.Strings(testKeys)
		require.Equal(t, testKeys, foundKeys, "Expected all keys to be found")
	})

	t.Run("early termination", func(t *testing.T) {
		fxClient, _, _ := prepareFixtures(t)
		testKeys := []string{"key1", "key2", "key3", "key4", "key5"}
		for _, key := range testKeys {
			err := fxClient.defaultStore.Set(context.Background(), key, []byte("value-"+key))
			require.NoError(t, err)
		}

		var foundKeys []string
		err := fxClient.defaultStore.Iterate(context.Background(), func(decryptor keyvaluestorage.Decryptor, key string, values []innerstorage.KeyValue) (bool, error) {
			foundKeys = append(foundKeys, key)
			return len(foundKeys) < 2, nil
		})
		require.NoError(t, err)
		require.Equal(t, 2, len(foundKeys), "expected to find exactly 2 keys before stopping")
	})

	t.Run("error during iteration", func(t *testing.T) {
		fxClient, _, _ := prepareFixtures(t)

		err := fxClient.defaultStore.Set(context.Background(), "test-key", []byte("test-value"))
		require.NoError(t, err)

		expectedErr := context.Canceled
		err = fxClient.defaultStore.Iterate(context.Background(), func(decryptor keyvaluestorage.Decryptor, key string, values []innerstorage.KeyValue) (bool, error) {
			return false, expectedErr
		})
		require.Equal(t, expectedErr, err, "expected error to be propagated")
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

func mapEqual[K comparable, V comparable](map1, map2 map[K]V) bool {
	if len(map1) != len(map2) {
		return false
	}
	for key, val1 := range map1 {
		if val2, ok := map2[key]; !ok || val1 != val2 {
			return false
		}
	}
	return true
}

var ctx = context.Background()

// countingSyncClient counts Broadcast calls; the store broadcasts once per
// applied Set/SetRaw, so the count observes how many applies happened.
type countingSyncClient struct {
	broadcasts atomic.Int32
}

func (c *countingSyncClient) Broadcast(ctx context.Context, objectId string, keyValues ...innerstorage.KeyValue) error {
	c.broadcasts.Add(1)
	return nil
}

type fixture struct {
	*keyValueService
	server     *rpctest.TestServer
	ts         *testServer
	syncClient *countingSyncClient
}

func newFixture(t *testing.T, keys *accountdata.AccountKeys, spacePayload spacestorage.SpaceStorageCreatePayload) *fixture {
	storePath := filepath.Join(t.TempDir(), "store.db")
	anyStore, err := anystore.Open(ctx, storePath, nil)
	require.NoError(t, err)
	storage, err := spacestorage.Create(ctx, anyStore, spacePayload)
	require.NoError(t, err)
	aclStorage, err := storage.AclStorage()
	require.NoError(t, err)
	aclList, err := list.BuildAclListWithIdentity(keys, aclStorage, recordverifier.NewValidateFull())
	require.NoError(t, err)
	storageId := "kv.storage"
	rpcHandler := rpctest.NewTestServer()
	syncClient := &countingSyncClient{}
	defaultStorage, err := keyvaluestorage.New(ctx,
		storageId,
		anyStore,
		storage.HeadStorage(),
		keys,
		syncClient,
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
	ts := &testServer{service: service, t: t}
	require.NoError(t, spacesyncproto.DRPCRegisterSpaceSync(rpcHandler, ts))
	return &fixture{
		keyValueService: service,
		server:          rpcHandler,
		ts:              ts,
		syncClient:      syncClient,
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
	service        *keyValueService
	t              *testing.T
	mu             sync.Mutex
	sent           []string // KeyPeerIds the server streamed back, in send order
	failTerminator bool
}

// setFailTerminator makes the server's terminator send fail, simulating a
// stream that breaks after the values were exchanged.
func (t *testServer) setFailTerminator(fail bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failTerminator = fail
}

// sentIds returns a copy of the order in which the server streamed values back.
func (t *testServer) sentIds() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.sent...)
}

func (t *testServer) StoreDiff(ctx context.Context, req *spacesyncproto.StoreDiffRequest) (*spacesyncproto.StoreDiffResponse, error) {
	return t.service.HandleStoreDiffRequest(ctx, req)
}

func (t *testServer) StoreElements(stream spacesyncproto.DRPCSpaceSync_StoreElementsStream) error {
	msg, err := stream.Recv()
	require.NoError(t.t, err)
	require.NotEmpty(t.t, msg.SpaceId)
	return t.service.HandleStoreElementsRequest(ctx, &recordingStream{DRPCSpaceSync_StoreElementsStream: stream, ts: t})
}

// recordingStream records the order of values the server sends, for ordering assertions.
type recordingStream struct {
	spacesyncproto.DRPCSpaceSync_StoreElementsStream
	ts *testServer
}

func (r *recordingStream) Send(kv *spacesyncproto.StoreKeyValue) error {
	r.ts.mu.Lock()
	if kv.KeyPeerId != "" {
		r.ts.sent = append(r.ts.sent, kv.KeyPeerId)
	} else if r.ts.failTerminator {
		r.ts.mu.Unlock()
		return errors.New("injected terminator send failure")
	}
	r.ts.mu.Unlock()
	return r.DRPCSpaceSync_StoreElementsStream.Send(kv)
}
