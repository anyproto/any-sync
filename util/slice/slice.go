package slice

import (
	"hash/fnv"
	"math/rand"
	"sort"

	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

func DifferenceRemovedAdded(a, b []string) (removed []string, added []string) {
	var amap = map[string]struct{}{}
	var bmap = map[string]struct{}{}

	for _, item := range a {
		amap[item] = struct{}{}
	}

	for _, item := range b {
		if _, exists := amap[item]; !exists {
			added = append(added, item)
		}
		bmap[item] = struct{}{}
	}

	for _, item := range a {
		if _, exists := bmap[item]; !exists {
			removed = append(removed, item)
		}
	}
	return
}

func FindPos(s []string, v string) int {
	for i, sv := range s {
		if sv == v {
			return i
		}
	}
	return -1
}

// Difference returns the elements in `a` that aren't in `b`.
func Difference(a, b []string) []string {
	var diff = make([]string, 0, len(a))
	for _, a1 := range a {
		if FindPos(b, a1) == -1 {
			diff = append(diff, a1)
		}
	}
	return diff
}

func Insert(s []string, pos int, v ...string) []string {
	if len(s) <= pos {
		return append(s, v...)
	}
	if pos == 0 {
		return append(v, s[pos:]...)
	}
	return append(s[:pos], append(v, s[pos:]...)...)
}

// Remove reuses provided slice capacity. Provided s slice should not be used after without reassigning to the func return!
func Remove(s []string, v string) []string {
	var n int
	for _, x := range s {
		if x != v {
			s[n] = x
			n++
		}
	}
	return s[:n]
}

func Filter(vals []string, cond func(string) bool) []string {
	var result = make([]string, 0, len(vals))
	for i := range vals {
		if cond(vals[i]) {
			result = append(result, vals[i])
		}
	}
	return result
}

func GetRandomString(s []string, seed string) string {
	rand.Seed(int64(hash(seed)))
	return s[rand.Intn(len(s))]
}

func hash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func SortedEquals(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

func UnsortedEquals(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}

	s1Sorted := make([]string, len(s1))
	s2Sorted := make([]string, len(s2))
	copy(s1Sorted, s1)
	copy(s2Sorted, s2)
	sort.Strings(s1Sorted)
	sort.Strings(s2Sorted)

	return SortedEquals(s1Sorted, s2Sorted)
}

func ContainsSorted[T constraints.Ordered](seq []T, subseq []T) bool {
	if len(seq) < len(subseq) {
		return false
	}
	slices.Sort(seq)
	slices.Sort(subseq)
	i := 0
	j := 0
	for i < len(seq) && j < len(subseq) {
		if seq[i] == subseq[j] {
			i++
			j++
		} else if seq[i] < subseq[j] {
			i++
		} else {
			// in this case we didn't find match for subseq[j] in seq
			return false
		}
	}
	// if we reached the end of subseq, then all elements were found
	return j == len(subseq)
}

func DiscardDuplicatesSorted[T comparable](sorted []T) []T {
	if len(sorted) <= 1 {
		return sorted
	}
	cnt := 1
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1] {
			sorted[cnt] = sorted[i]
			cnt++
		}
	}
	return sorted[:cnt]
}

func RemoveRepeatedSorted[T constraints.Ordered](nums []T) []T {
	if len(nums) == 0 {
		return nums
	}
	writeIndex := 0
	i := 0
	for i < len(nums) {
		count := 1
		for i+1 < len(nums) && nums[i] == nums[i+1] {
			count++
			i++
		}
		if count == 1 {
			nums[writeIndex] = nums[i]
			writeIndex++
		}
		i++
	}
	return nums[:writeIndex]
}

func RemoveUniqueElementsSorted[T constraints.Ordered](nums []T) []T {
	if len(nums) == 0 {
		return nums
	}
	writeIndex := 0
	i := 0
	for i < len(nums) {
		count := 1
		for i+1 < len(nums) && nums[i] == nums[i+1] {
			count++
			i++
		}
		if count > 1 {
			for j := 0; j < count; j++ {
				nums[writeIndex] = nums[i]
				writeIndex++
			}
		}
		i++
	}
	return nums[:writeIndex]
}

func DiscardDuplicatesSortedFunc[T any](sorted []T, equal func(T, T) bool) []T {
	if len(sorted) <= 1 {
		return sorted
	}
	cnt := 1
	for i := 1; i < len(sorted); i++ {
		if !equal(sorted[i], sorted[i-1]) {
			sorted[cnt] = sorted[i]
			cnt++
		}
	}
	return sorted[:cnt]
}

func DiscardFromSlice[T any](elements []T, isDiscarded func(T) bool) []T {
	var (
		finishedIdx = 0
		currentIdx  = 0
	)
	for currentIdx < len(elements) {
		if !isDiscarded(elements[currentIdx]) {
			if finishedIdx != currentIdx {
				elements[finishedIdx] = elements[currentIdx]
			}
			finishedIdx++
		}
		currentIdx++
	}
	elements = elements[:finishedIdx]
	return elements
}

func CompareMaps[T comparable](map1, map2 map[T]struct{}) (both, first, second []T) {
	both = []T{}
	first = []T{}
	second = []T{}

	for key := range map1 {
		if _, found := map2[key]; found {
			both = append(both, key)
		} else {
			first = append(first, key)
		}
	}

	for key := range map2 {
		if _, found := map1[key]; !found {
			second = append(second, key)
		}
	}

	return both, first, second
}
