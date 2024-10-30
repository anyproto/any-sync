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

func TestContainsSorted(t *testing.T) {
	tests := []struct {
		name     string
		first    []int
		second   []int
		expected bool
	}{
		{"both empty", []int{}, []int{}, true},
		{"first empty", []int{}, []int{1}, false},
		{"equal length but different", []int{2}, []int{1}, false},
		{"second empty", []int{1, 2, 3}, []int{}, true},
		{"both non-empty and first contains second", []int{1, 2, 3, 4, 5}, []int{2, 3, 4}, true},
		{"both non-empty and first does not contain second", []int{1, 2, 3, 4, 5}, []int{3, 4, 6}, false},
		{"both non-empty and first shorter than second", []int{1, 2, 3}, []int{1, 2, 3, 4}, false},
		{"both non-empty and first equals second", []int{1, 2, 3}, []int{1, 2, 3}, true},
		{"both non-empty and first contains second at the beginning", []int{1, 2, 3, 4, 5}, []int{1, 2, 3}, true},
		{"both non-empty and first contains second at the end", []int{1, 2, 3, 4, 5}, []int{3, 4, 5}, true},
		{"non-consecutive elements", []int{1, 3, 5, 7, 9}, []int{3, 7}, true},
		{"unsorted first contains sorted second", []int{5, 1, 3, 2, 4}, []int{2, 3, 4}, true},
		{"unsorted first does not contain sorted second", []int{5, 1, 3, 2, 4}, []int{3, 4, 6}, false},
		{"test previous bug", []int{1, 3}, []int{2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsSorted(tt.first, tt.second)
			if result != tt.expected {
				t.Errorf("ContainsSorted(%v, %v) = %v; want %v", tt.first, tt.second, result, tt.expected)
			}
		})
	}
}
