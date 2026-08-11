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

	"github.com/anyproto/any-sync/app"
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
		fxClient.limiter.Close(ctx)
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
		fxClient.limiter.Close(ctx)
		fxClient.check(t, "key1", []byte("value1"))
		fxClient.check(t, "key1", []byte("value2"))
		fxServer.check(t, "key1", []byte("value1"))
		fxServer.check(t, "key1", []byte("value2"))
		fxClient.add(t, "key1", []byte("value1-2"))
		fxServer.add(t, "key1", []byte("value2-2"))
		err = fxClient.SyncWithPeer(serverPeer)
		require.NoError(t, err)
		fxClient.limiter.Close(ctx)
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
		fxClient.limiter.Close(ctx)

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

// rawWatermark hand-builds a signed deletion watermark, letting a test send
// one from an arbitrary identity.
func rawWatermark(t *testing.T, keys *accountdata.AccountKeys, prefix string, ts int64) *spacesyncproto.StoreKeyValue {
	protoPeerKey, err := keys.PeerKey.GetPublic().Marshall()
	require.NoError(t, err)
	protoIdentityKey, err := keys.SignKey.GetPublic().Marshall()
	require.NoError(t, err)
	inner := spacesyncproto.StoreKeyInner{
		Peer:           protoPeerKey,
		Identity:       protoIdentityKey,
		TimestampMicro: ts,
		AclHeadId:      "acl-head",
		Key:            prefix,
		Delete:         &spacesyncproto.StoreDeletePrefix{Prefix: prefix},
	}
	innerBytes, err := inner.MarshalVT()
	require.NoError(t, err)
	peerSig, err := keys.PeerKey.Sign(innerBytes)
	require.NoError(t, err)
	identitySig, err := keys.SignKey.Sign(innerBytes)
	require.NoError(t, err)
	return &spacesyncproto.StoreKeyValue{
		KeyPeerId:         prefix + "-" + keys.PeerKey.GetPublic().PeerId(),
		Value:             innerBytes,
		PeerSignature:     peerSig,
		IdentitySignature: identitySig,
	}
}

// prefixIds lists the raw stored row ids under a key prefix, watermark rows
// included.
func prefixIds(t *testing.T, store keyvaluestorage.Storage, prefix string) []string {
	var ids []string
	err := store.InnerStorage().IteratePrefix(ctx, prefix, func(kv innerstorage.KeyValue) error {
		ids = append(ids, kv.KeyPeerId)
		return nil
	})
	require.NoError(t, err)
	sort.Strings(ids)
	return ids
}

func TestDeletePrefix(t *testing.T) {
	t.Run("drops locally, propagates via sync, rejects resurrection", func(t *testing.T) {
		fxClient, fxServer, serverPeer := prepareFixtures(t)
		fxClient.add(t, "read/sp1/a", []byte("va"))
		fxClient.add(t, "read/sp1/b", []byte("vb"))
		fxClient.add(t, "other/c", []byte("vc"))
		fxServer.add(t, "read/sp1/d", []byte("vd"))
		// syncWithPeer directly: the limiter permits one scheduled sync per
		// peer and its Close is terminal, while this test needs two rounds.
		require.NoError(t, fxClient.keyValueService.syncWithPeer(ctx, serverPeer))
		require.Len(t, prefixIds(t, fxServer.defaultStore, "read/sp1/"), 3)

		// Capture the pre-delete rows: a device restoring an old snapshot
		// would push exactly these.
		var stale []*spacesyncproto.StoreKeyValue
		err := fxServer.defaultStore.InnerStorage().IteratePrefix(ctx, "read/sp1/", func(kv innerstorage.KeyValue) error {
			p := kv.Proto()
			p.Value = append([]byte(nil), p.Value...)
			p.PeerSignature = append([]byte(nil), p.PeerSignature...)
			p.IdentitySignature = append([]byte(nil), p.IdentitySignature...)
			stale = append(stale, p)
			return nil
		})
		require.NoError(t, err)
		require.Len(t, stale, 3)

		require.NoError(t, fxClient.defaultStore.DeletePrefix(ctx, "read/sp1/"))
		require.NoError(t, fxClient.keyValueService.syncWithPeer(ctx, serverPeer))

		for _, fx := range []*fixture{fxClient, fxServer} {
			ids := prefixIds(t, fx.defaultStore, "read/sp1/")
			require.Len(t, ids, 1, "only the watermark row remains: %v", ids)
			require.True(t, strings.HasPrefix(ids[0], "read/sp1/-"), "remaining row is the watermark: %v", ids)
			require.False(t, fx.check(t, "read/sp1/a", []byte("va")))
			require.False(t, fx.check(t, "read/sp1/d", []byte("vd")))
			require.True(t, fx.check(t, "other/c", []byte("vc")), "rows outside the prefix survive")
			// The public read surface hides the watermark row.
			var seen []string
			require.NoError(t, fx.defaultStore.Iterate(ctx, func(_ keyvaluestorage.Decryptor, key string, _ []innerstorage.KeyValue) (bool, error) {
				seen = append(seen, key)
				return true, nil
			}))
			require.Equal(t, []string{"other/c"}, seen)
		}

		// Resurrection attempt: replaying the captured pre-delete rows is a
		// no-op on both replicas.
		require.NoError(t, fxServer.defaultStore.SetRaw(ctx, stale...))
		require.Len(t, prefixIds(t, fxServer.defaultStore, "read/sp1/"), 1)
		require.NoError(t, fxClient.defaultStore.SetRaw(ctx, stale...))
		require.Len(t, prefixIds(t, fxClient.defaultStore, "read/sp1/"), 1)
	})

	t.Run("watermark from a non-owner identity is rejected", func(t *testing.T) {
		fxClient, _, _ := prepareFixtures(t)
		fxClient.add(t, "read/sp1/a", []byte("va"))

		strangerKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		wm := rawWatermark(t, strangerKeys, "read/sp1/", time.Now().Add(time.Hour).UnixMicro())
		require.NoError(t, fxClient.defaultStore.SetRaw(ctx, wm))

		require.True(t, fxClient.check(t, "read/sp1/a", []byte("va")), "rows survive a stranger's watermark")
		require.Len(t, prefixIds(t, fxClient.defaultStore, "read/sp1/"), 1, "the stranger's watermark row must not be stored")
	})
}

// recordingIndexer records Index/RemoveIndex calls; RemoveIndex makes it
// deletion-aware.
type recordingIndexer struct {
	mu      sync.Mutex
	indexed []string
	removed []string
}

func (r *recordingIndexer) Init(a *app.App) error { return nil }
func (r *recordingIndexer) Name() string          { return keyvaluestorage.IndexerCName }

func (r *recordingIndexer) Index(_ keyvaluestorage.Decryptor, keyValues ...innerstorage.KeyValue) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, kv := range keyValues {
		r.indexed = append(r.indexed, kv.KeyPeerId)
	}
	return nil
}

func (r *recordingIndexer) RemoveIndex(keyPeerIds ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed = append(r.removed, keyPeerIds...)
	return nil
}

func (r *recordingIndexer) counts() (indexed, removed int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.indexed), len(r.removed)
}

// newBareStorage builds a keyvaluestorage.Storage without the rpc service
// around it, with an arbitrary indexer.
func newBareStorage(t *testing.T, keys *accountdata.AccountKeys, spacePayload spacestorage.SpaceStorageCreatePayload, indexer keyvaluestorage.Indexer) keyvaluestorage.Storage {
	anyStore, err := anystore.Open(ctx, filepath.Join(t.TempDir(), "store.db"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = anyStore.Close() })
	storage, err := spacestorage.Create(ctx, anyStore, spacePayload)
	require.NoError(t, err)
	aclStorage, err := storage.AclStorage()
	require.NoError(t, err)
	aclList, err := list.BuildAclListWithIdentity(keys, aclStorage, recordverifier.NewValidateFull())
	require.NoError(t, err)
	st, err := keyvaluestorage.New(ctx, "kv.storage", anyStore, storage.HeadStorage(), keys, &countingSyncClient{}, aclList, indexer)
	require.NoError(t, err)
	return st
}

// exportPrefix returns the wire protos of every raw row under the prefix.
func exportPrefix(t *testing.T, store keyvaluestorage.Storage, prefix string) []*spacesyncproto.StoreKeyValue {
	var protos []*spacesyncproto.StoreKeyValue
	err := store.InnerStorage().IteratePrefix(ctx, prefix, func(kv innerstorage.KeyValue) error {
		protos = append(protos, kv.Proto())
		return nil
	})
	require.NoError(t, err)
	return protos
}

// TestSetCoveredByWatermark: a local write under a newer watermark fails
// loudly instead of returning success for a value every replica would drop.
func TestSetCoveredByWatermark(t *testing.T) {
	fxClient, _, _ := prepareFixtures(t)
	_, err := fxClient.defaultStore.InnerStorage().Set(ctx, innerstorage.KeyValue{
		KeyPeerId:      "read/sp1/-peerX",
		Key:            "read/sp1/",
		PeerId:         "peerX",
		Identity:       "identity",
		DeletePrefix:   "read/sp1/",
		TimestampMicro: time.Now().Add(time.Hour).UnixMicro(),
	})
	require.NoError(t, err)

	err = fxClient.defaultStore.Set(ctx, "read/sp1/a", []byte("va"))
	require.ErrorIs(t, err, keyvaluestorage.ErrCoveredByWatermark)
	require.ErrorIs(t, fxClient.defaultStore.DeletePrefix(ctx, "read/sp1/sub/"), keyvaluestorage.ErrCoveredByWatermark,
		"a narrower watermark under a newer covering one is refused too")
	require.NoError(t, fxClient.defaultStore.Set(ctx, "other/b", []byte("vb")), "keys outside the prefix write normally")
}

func TestDeletePrefixEmpty(t *testing.T) {
	fxClient, _, _ := prepareFixtures(t)
	require.ErrorIs(t, fxClient.defaultStore.DeletePrefix(ctx, ""), keyvaluestorage.ErrEmptyDeletePrefix)
}

// TestIndexerDeletionFlow pins the indexer contract around watermarks:
// applied rows index, watermark-dropped rows un-index via the optional
// DeletionAwareIndexer, and rows the store rejects never reach Index.
func TestIndexerDeletionFlow(t *testing.T) {
	firstKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	secondKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	secondKeys.SignKey = firstKeys.SignKey
	payload := newStorageCreatePayload(t, firstKeys)

	idx := &recordingIndexer{}
	stA := newBareStorage(t, firstKeys, payload, keyvaluestorage.NoOpIndexer{})
	stB := newBareStorage(t, secondKeys, payload, idx)

	require.NoError(t, stA.Set(ctx, "read/sp1/a", []byte("va")))
	require.NoError(t, stA.Set(ctx, "read/sp1/b", []byte("vb")))
	stale := exportPrefix(t, stA, "read/sp1/")
	require.Len(t, stale, 2)

	require.NoError(t, stB.SetRaw(ctx, stale...))
	indexed, removed := idx.counts()
	require.Equal(t, 2, indexed, "applied rows index")
	require.Equal(t, 0, removed)

	require.NoError(t, stA.DeletePrefix(ctx, "read/sp1/"))
	wmProtos := exportPrefix(t, stA, "read/sp1/")
	require.Len(t, wmProtos, 1, "only the watermark row remains on A")
	require.NoError(t, stB.SetRaw(ctx, wmProtos...))
	indexed, removed = idx.counts()
	require.Equal(t, 2, indexed, "the watermark row itself is not indexed")
	require.Equal(t, 2, removed, "dropped rows are un-indexed")

	// Resurrection attempt: the rejected rows must not reach Index.
	require.NoError(t, stB.SetRaw(ctx, stale...))
	indexed, _ = idx.counts()
	require.Equal(t, 2, indexed, "rejected rows must not be indexed")
}

// attachPeer wires fx to hub's rpc server and returns the peer fx dials.
func attachPeer(t *testing.T, fx *fixture, fxKeys *accountdata.AccountKeys, hub *fixture, hubKeys *accountdata.AccountKeys) peer.Peer {
	hubConn, fxConn := rpctest.MultiConnPair(fxKeys.PeerId, hubKeys.PeerId)
	hubPeer, err := peer.NewPeer(hubConn, fx.server)
	require.NoError(t, err)
	_, err = peer.NewPeer(fxConn, hub.server)
	require.NoError(t, err)
	return hubPeer
}

// TestDeletePrefix_OfflinePeers covers devices that were offline across a
// prefix deletion:
//   - stale unsynced rows (older than the watermark) are rejected by the hub
//     and dropped by their writer once the watermark reaches it;
//   - rows written after the deletion (newer than the watermark) win by LWW
//     and propagate — the app-level reconciler's re-issued delete is what
//     removes them, and does so everywhere.
func TestDeletePrefix_OfflinePeers(t *testing.T) {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	hubKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	hubKeys.SignKey = ownerKeys.SignKey
	staleKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	staleKeys.SignKey = ownerKeys.SignKey
	lateKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	lateKeys.SignKey = ownerKeys.SignKey
	payload := newStorageCreatePayload(t, ownerKeys)

	fxOwner := newFixture(t, ownerKeys, payload)
	fxHub := newFixture(t, hubKeys, payload)
	fxStale := newFixture(t, staleKeys, payload)
	fxLate := newFixture(t, lateKeys, payload)
	hubForOwner := attachPeer(t, fxOwner, ownerKeys, fxHub, hubKeys)
	hubForStale := attachPeer(t, fxStale, staleKeys, fxHub, hubKeys)
	hubForLate := attachPeer(t, fxLate, lateKeys, fxHub, hubKeys)

	// Stale device writes while offline; owner writes and syncs.
	fxStale.add(t, "read/sp1/x", []byte("vx"))
	fxStale.add(t, "read/sp1/y", []byte("vy"))
	fxStale.add(t, "notes/keep", []byte("vk"))
	fxOwner.add(t, "read/sp1/a", []byte("va"))
	require.NoError(t, fxOwner.keyValueService.syncWithPeer(ctx, hubForOwner))

	// Timestamps order the whole scenario; keep them strictly increasing.
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, fxOwner.defaultStore.DeletePrefix(ctx, "read/sp1/"))
	require.NoError(t, fxOwner.keyValueService.syncWithPeer(ctx, hubForOwner))
	require.Len(t, prefixIds(t, fxHub.defaultStore, "read/sp1/"), 1, "hub holds only the watermark")

	// The stale device comes online: one round pushes its rows (rejected)
	// and pulls the watermark (drops its local copies).
	require.NoError(t, fxStale.keyValueService.syncWithPeer(ctx, hubForStale))
	require.Len(t, prefixIds(t, fxStale.defaultStore, "read/sp1/"), 1, "stale device drops its unsynced rows on receiving the watermark")
	require.Len(t, prefixIds(t, fxHub.defaultStore, "read/sp1/"), 1, "stale rows must not stick on the hub")
	require.True(t, fxStale.check(t, "notes/keep", []byte("vk")), "unsynced rows outside the prefix survive")
	require.True(t, fxHub.check(t, "notes/keep", []byte("vk")), "out-of-prefix rows still sync")

	// A device that writes AFTER the deletion (its clock is past the
	// watermark) wins by LWW: the row propagates by design.
	time.Sleep(2 * time.Millisecond)
	fxLate.add(t, "read/sp1/z", []byte("vz"))
	require.NoError(t, fxLate.keyValueService.syncWithPeer(ctx, hubForLate))
	require.Len(t, prefixIds(t, fxHub.defaultStore, "read/sp1/"), 2, "post-deletion write survives the old watermark")

	// The reconciler's re-issued delete removes the late row everywhere.
	require.NoError(t, fxOwner.keyValueService.syncWithPeer(ctx, hubForOwner))
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, fxOwner.defaultStore.DeletePrefix(ctx, "read/sp1/"))
	require.NoError(t, fxOwner.keyValueService.syncWithPeer(ctx, hubForOwner))
	require.NoError(t, fxLate.keyValueService.syncWithPeer(ctx, hubForLate))
	for name, fx := range map[string]*fixture{"owner": fxOwner, "hub": fxHub, "late": fxLate} {
		ids := prefixIds(t, fx.defaultStore, "read/sp1/")
		require.Len(t, ids, 1, "%s converges to the watermark only: %v", name, ids)
	}
}
