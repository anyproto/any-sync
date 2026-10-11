package ldiff

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// cachedRootRemote models any-sync-node's tryNodeHeadSync optimization:
// a hash-only root response carries Count=1 even for a large space.
// Explicit element requests and non-root ranges carry real counts.
type cachedRootRemote struct {
	diff                Diff
	calls               int
	rootElementRequests int
	returnedElements    int
}

func (r *cachedRootRemote) Ranges(ctx context.Context, ranges []Range, buf []RangeResult) ([]RangeResult, error) {
	r.calls++
	results, err := r.diff.Ranges(ctx, ranges, buf)
	if err != nil {
		return nil, err
	}
	for i, rg := range ranges {
		if rg.From == 0 && rg.To == math.MaxUint64 {
			if rg.Elements {
				r.rootElementRequests++
			} else if len(ranges) == 1 {
				results[i].Count = 1
				results[i].Elements = nil
			}
		}
		r.returnedElements += len(results[i].Elements)
	}
	return results, nil
}

func rootCountElements(n int) []Element {
	result := make([]Element, n)
	for i := range result {
		result[i] = Element{Id: fmt.Sprintf("id_%08d", i), Head: fmt.Sprintf("head_%08d", i)}
	}
	return result
}

func TestDiff_CachedRootCountCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		local, remote        int
		wantNew, wantRemoved int
		allowRootElements    bool
	}{
		{"large_space_seven_new", 71295, 71302, 7, 0, false},
		{"genuine_single_remote_element", 4096, 1, 0, 4095, false},
		{"equal_hash_large_space", 4096, 4096, 0, 0, false},
		{"empty_local", 0, 3, 3, 0, true},
		{"small_local", 3, 4, 1, 0, true},
		{"threshold_local", 256, 257, 1, 0, true},
		{"above_threshold_local", 257, 258, 1, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local := New(32, 256)
			other := New(32, 256)
			local.Set(rootCountElements(tc.local)...)
			other.Set(rootCountElements(tc.remote)...)
			localHash, remoteHash := local.Hash(), other.Hash()
			remote := &cachedRootRemote{diff: other}
			added, changed, removed, err := local.Diff(context.Background(), remote)
			require.NoError(t, err)
			require.Len(t, added, tc.wantNew)
			require.Empty(t, changed)
			require.Len(t, removed, tc.wantRemoved)
			require.Equal(t, localHash, local.Hash(), "comparison must not change the local space hash")
			require.Equal(t, remoteHash, other.Hash(), "comparison must not change the remote space hash")
			if !tc.allowRootElements {
				require.Zero(t, remote.rootElementRequests, "a cached Count=1 must not request every head in a large space")
			}
			if tc.name == "large_space_seven_new" {
				require.Less(t, remote.returnedElements, 7*256, "only differing leaf ranges should transfer elements")
				for _, id := range added {
					_, err := local.Element(id)
					require.ErrorIs(t, err, ErrElementNotFound)
					_, err = other.Element(id)
					require.NoError(t, err)
				}
			}
			if tc.name == "equal_hash_large_space" {
				require.Equal(t, 1, remote.calls)
				require.Zero(t, remote.returnedElements)
			}
		})
	}
}

func TestCompareDiff_CachedRootCountCompatibility(t *testing.T) {
	local := New(32, 256).(CompareDiff)
	other := New(32, 256)
	local.Set(rootCountElements(4096)...)
	other.Set(rootCountElements(4103)...)
	// Verify CompareDiff preserves head ordering as well as new-ID discovery.
	other.Set(Element{Id: "id_00000000", Head: "zz_remote_newer"})
	local.Set(Element{Id: "id_00000001", Head: "zz_local_newer"})
	remote := &cachedRootRemote{diff: other}
	added, ourChanged, theirChanged, removed, err := local.CompareDiff(context.Background(), remote)
	require.NoError(t, err)
	require.Len(t, added, 7)
	require.Equal(t, []string{"id_00000001"}, ourChanged)
	require.Equal(t, []string{"id_00000000"}, theirChanged)
	require.Empty(t, removed)
	require.Zero(t, remote.rootElementRequests)
}
