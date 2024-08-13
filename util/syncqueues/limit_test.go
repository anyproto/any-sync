package syncqueues

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLimitExcluded(t *testing.T) {
	for _, tc := range []struct {
		peerStep      []int
		excludeIds    []string
		excludedLimit int
	}{
		{
			peerStep:      []int{5, 4, 3, 2, 1},
			excludeIds:    []string{"excluded1", "excluded2"},
			excludedLimit: 10,
		},
	} {
		totalStep := make([]int, len(tc.peerStep)-1)
		totalStep[0] = tc.peerStep[0]
		for i := 1; i < len(tc.peerStep)-2; i++ {
			totalStep[i] = totalStep[i-1] + tc.peerStep[i] + 1
		}
		totalStep[len(totalStep)-1] = totalStep[len(totalStep)-2] + tc.peerStep[len(tc.peerStep)-1]
		l := NewLimit(tc.peerStep, totalStep, tc.excludeIds, tc.excludedLimit)

		// Test regular peers
		for j := 0; j < len(tc.peerStep); j++ {
			for i := 0; i < tc.peerStep[j]; i++ {
				require.True(t, l.Take(fmt.Sprint(j)))
			}
			require.False(t, l.Take(fmt.Sprint(j)))
		}
		require.Equal(t, len(tc.peerStep)-1, l.counter)
		require.Equal(t, totalStep[len(totalStep)-1], l.total)

		// Test excluded peers
		for _, id := range tc.excludeIds {
			for i := 0; i < tc.excludedLimit; i++ {
				require.True(t, l.Take(id))
			}
			require.False(t, l.Take(id))
		}

		// Release regular peers
		for j := 0; j < len(tc.peerStep); j++ {
			for i := 0; i < tc.peerStep[j]; i++ {
				l.Release(fmt.Sprint(j))
			}
		}
		require.Equal(t, 0, l.counter)
		require.Equal(t, 0, l.total)

		// Release excluded peers
		for _, id := range tc.excludeIds {
			for i := 0; i < tc.excludedLimit; i++ {
				l.Release(id)
			}
		}
		require.Equal(t, 0, l.excludedTotal)
	}
}
