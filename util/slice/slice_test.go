package slice

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func TestCompareMaps(t *testing.T) {
	tests := []struct {
		name           string
		map1, map2     map[string]struct{}
		expectedBoth   []string
		expectedFirst  []string
		expectedSecond []string
	}{
		{
			name:           "Both maps empty",
			map1:           map[string]struct{}{},
			map2:           map[string]struct{}{},
			expectedBoth:   []string{},
			expectedFirst:  []string{},
			expectedSecond: []string{},
		},
		{
			name: "Disjoint maps",
			map1: map[string]struct{}{
				"a": {},
				"b": {},
			},
			map2: map[string]struct{}{
				"c": {},
				"d": {},
			},
			expectedBoth:   []string{},
			expectedFirst:  []string{"a", "b"},
			expectedSecond: []string{"c", "d"},
		},
		{
			name: "Identical maps",
			map1: map[string]struct{}{
				"a": {},
				"b": {},
			},
			map2: map[string]struct{}{
				"a": {},
				"b": {},
			},
			expectedBoth:   []string{"a", "b"},
			expectedFirst:  []string{},
			expectedSecond: []string{},
		},
		{
			name: "Partial overlap",
			map1: map[string]struct{}{
				"a": {},
				"b": {},
				"c": {},
			},
			map2: map[string]struct{}{
				"b": {},
				"c": {},
				"d": {},
			},
			expectedBoth:   []string{"b", "c"},
			expectedFirst:  []string{"a"},
			expectedSecond: []string{"d"},
		},
		{
			name: "First map empty",
			map1: map[string]struct{}{},
			map2: map[string]struct{}{
				"a": {},
				"b": {},
			},
			expectedBoth:   []string{},
			expectedFirst:  []string{},
			expectedSecond: []string{"a", "b"},
		},
		{
			name: "Second map empty",
			map1: map[string]struct{}{
				"a": {},
				"b": {},
			},
			map2:           map[string]struct{}{},
			expectedBoth:   []string{},
			expectedFirst:  []string{"a", "b"},
			expectedSecond: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			both, onlyInFirst, onlyInSecond := CompareMaps(tt.map1, tt.map2)
			slices.Sort(onlyInFirst)
			slices.Sort(onlyInSecond)
			slices.Sort(both)
			require.Equal(t, tt.expectedBoth, both)
			require.Equal(t, tt.expectedFirst, onlyInFirst)
			require.Equal(t, tt.expectedSecond, onlyInSecond)
		})
	}
}
