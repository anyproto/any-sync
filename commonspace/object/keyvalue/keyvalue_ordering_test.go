package keyvalue

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app/ldiff"
)

// TestSortIdsNewestFirst asserts ids are ordered by their diff Head (big-endian
// timestamp) descending, ties broken by id ascending, and ids missing from the
// diff placed last — so the stream order is fully deterministic.
func TestSortIdsNewestFirst(t *testing.T) {
	diff := ldiff.New(16, 16)
	diff.Set(
		ldiff.Element{Id: "old", Head: "\x00\x01"},
		ldiff.Element{Id: "new", Head: "\x00\x03"},
		ldiff.Element{Id: "tie-b", Head: "\x00\x02"},
		ldiff.Element{Id: "tie-a", Head: "\x00\x02"},
	)
	ids := []string{"tie-b", "missing", "old", "new", "tie-a"}
	sortIdsNewestFirst(diff, ids)
	require.Equal(t, []string{"new", "tie-a", "tie-b", "old", "missing"}, ids)
}

// keyOf extracts the logical key from a "<key>-<peerId>" id by stripping the
// final "-<peerId>" segment: peer ids contain no dash, keys may (e.g. key-003).
func keyOf(id string) string {
	if i := strings.LastIndex(id, "-"); i >= 0 {
		return id[:i]
	}
	return id
}

// TestStoreElementsNewestFirst asserts the server streams requested values
// newest-first (by timestamp desc), and that the client still converges to all.
func TestStoreElementsNewestFirst(t *testing.T) {
	fxClient, fxServer, serverPeer := prepareFixtures(t)
	keys := []string{"k0", "k1", "k2", "k3", "k4"}
	for _, k := range keys {
		fxServer.add(t, k, []byte("v-"+k))
		time.Sleep(2 * time.Millisecond) // strictly increasing write timestamps
	}

	require.NoError(t, fxClient.SyncWithPeer(serverPeer))
	fxClient.limiter.Close()

	var sentKeys []string
	for _, id := range fxServer.ts.sentIds() {
		sentKeys = append(sentKeys, keyOf(id))
	}
	require.Equal(t, []string{"k4", "k3", "k2", "k1", "k0"}, sentKeys, "server must stream newest-first")

	// Ordering must not affect convergence — the client still gets every value.
	for _, k := range keys {
		require.True(t, fxClient.check(t, k, []byte("v-"+k)), "client should converge to all values")
	}
	require.Equal(t,
		fxServer.defaultStore.InnerStorage().Diff().Hash(),
		fxClient.defaultStore.InnerStorage().Diff().Hash(),
		"head-hash must converge regardless of stream order")
}

// TestPushedValuesPersistDespiteSendFailure asserts the server saves the values
// a peer pushed even when streaming its own response fails: the pushed values
// were already received and are independent of the response stream.
func TestPushedValuesPersistDespiteSendFailure(t *testing.T) {
	fxClient, fxServer, serverPeer := prepareFixtures(t)
	fxClient.add(t, "pushed", []byte("pushed-value"))
	fxServer.ts.setFailTerminator(true)

	require.NoError(t, fxClient.SyncWithPeer(serverPeer))
	fxClient.limiter.Close()

	require.True(t, fxServer.check(t, "pushed", []byte("pushed-value")),
		"server must persist pushed values even when its response send fails")
}

// TestIncrementalApplyConvergence drives a pull larger than applyBatchSize so the
// client applies multiple batches, and asserts full convergence (batched apply
// is equivalent to one-shot apply for the final state).
func TestIncrementalApplyConvergence(t *testing.T) {
	fxClient, fxServer, serverPeer := prepareFixtures(t)
	const n = applyBatchSize + 50 // span more than one apply batch
	for i := 0; i < n; i++ {
		fxServer.add(t, fmt.Sprintf("key-%03d", i), []byte(fmt.Sprintf("val-%03d", i)))
	}

	require.NoError(t, fxClient.SyncWithPeer(serverPeer))
	fxClient.limiter.Close()

	// The client broadcasts once per applied SetRaw, so the broadcast count
	// proves the pull was applied incrementally rather than in one shot.
	expectedBatches := int32((n + applyBatchSize - 1) / applyBatchSize)
	require.Equal(t, expectedBatches, fxClient.syncClient.broadcasts.Load(),
		"client must apply the pull in ceil(n/applyBatchSize) batches")

	for i := 0; i < n; i++ {
		require.True(t, fxClient.check(t, fmt.Sprintf("key-%03d", i), []byte(fmt.Sprintf("val-%03d", i))),
			"value %d should have been applied", i)
	}
	// Batched apply must reach the same head-hash as the server (full convergence).
	require.Equal(t,
		fxServer.defaultStore.InnerStorage().Diff().Hash(),
		fxClient.defaultStore.InnerStorage().Diff().Hash(),
		"head-hash must converge after multi-batch apply")
}
