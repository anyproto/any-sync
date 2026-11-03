package ldiff

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/cespare/xxhash"
	"github.com/stretchr/testify/require"
)

// requireCanonical checks that the range tree of d equals a fresh build of the same elements:
// same ranges, same division, counts and hashes per range, no orphans, root count equal to Len
func requireCanonical(t testing.TB, d *diff, label string) {
	t.Helper()
	fresh := newDiff(d.divideFactor, d.compareThreshold).(*diff)
	fresh.Set(d.Elements()...)
	require.Emptyf(t, d.ranges.dirty, "%s: dirty ranges after recalculation", label)
	require.Equalf(t, d.sl.Len(), d.ranges.topRange.elements, "%s: root count differs from Len", label)
	require.Equalf(t, fresh.Hash(), d.Hash(), "%s: hash differs from fresh build", label)
	for tuple := range d.ranges.ranges {
		if _, ok := fresh.ranges.ranges[tuple]; !ok {
			t.Fatalf("%s: range %x-%x is not in a fresh build", label, tuple.from, tuple.to)
		}
	}
	for tuple, want := range fresh.ranges.ranges {
		got, ok := d.ranges.ranges[tuple]
		if !ok {
			t.Fatalf("%s: range %x-%x missing", label, tuple.from, tuple.to)
		}
		require.Equalf(t, want.isDivided, got.isDivided, "%s: range %x-%x: isDivided", label, tuple.from, tuple.to)
		require.Equalf(t, want.elements, got.elements, "%s: range %x-%x: elements", label, tuple.from, tuple.to)
		require.Equalf(t, want.level, got.level, "%s: range %x-%x: level", label, tuple.from, tuple.to)
		require.Equalf(t, hex.EncodeToString(want.hash), hex.EncodeToString(got.hash), "%s: range %x-%x: hash", label, tuple.from, tuple.to)
		if got == d.ranges.topRange {
			continue
		}
		require.NotNilf(t, got.parent, "%s: range %x-%x: no parent", label, tuple.from, tuple.to)
		parent := d.ranges.ranges[rangeTuple{from: got.parent.from, to: got.parent.to}]
		require.Samef(t, got.parent, parent, "%s: range %x-%x: parent not in ranges", label, tuple.from, tuple.to)
		require.Truef(t, parent.isDivided, "%s: range %x-%x: parent not divided", label, tuple.from, tuple.to)
	}
}

func requireElements(t testing.TB, d Diff, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	for _, el := range d.Elements() {
		got[el.Id] = el.Head
	}
	require.Equal(t, want, got)
	require.Equal(t, len(want), d.Len())
}

// idsMatching returns n ids whose id hash satisfies match
func idsMatching(n int, match func(hash uint64) bool) (ids []string) {
	for i := 0; len(ids) < n; i++ {
		id := fmt.Sprintf("p%d", i)
		if match(xxhash.Sum64([]byte(id))) {
			ids = append(ids, id)
		}
	}
	return
}

// idsWithPrefix returns n ids whose id hash starts with the given top bits
func idsWithPrefix(prefix uint64, bits uint, n int) []string {
	return idsMatching(n, func(hash uint64) bool { return hash>>(64-bits) == prefix })
}

func prefixTuple(prefix uint64, bits uint) rangeTuple {
	size := uint64(1) << (64 - bits)
	from := prefix << (64 - bits)
	return rangeTuple{from: from, to: from + size - 1}
}

// idSet holds live ids for random picks
type idSet struct {
	ids []string
	idx map[string]int
}

func newIdSet() *idSet {
	return &idSet{idx: map[string]int{}}
}

func (s *idSet) add(id string) {
	if _, ok := s.idx[id]; !ok {
		s.idx[id] = len(s.ids)
		s.ids = append(s.ids, id)
	}
}

func (s *idSet) del(id string) {
	i := s.idx[id]
	last := s.ids[len(s.ids)-1]
	s.ids[i] = last
	s.idx[last] = i
	s.ids = s.ids[:len(s.ids)-1]
	delete(s.idx, id)
}

func (s *idSet) pick(rng *rand.Rand) string {
	return s.ids[rng.Intn(len(s.ids))]
}

type canonicalConfig struct {
	df, th int
	// clustered ids share this many top hash bits: a chain of divided ranges that merges level by level
	clusterBits uint
	// 0: sixteen checks per phase
	checkEvery int
}

func TestHashRanges_RandomOpsStayCanonical(t *testing.T) {
	for _, c := range []canonicalConfig{
		{df: 4, th: 4, clusterBits: 6, checkEvery: 1},
		{df: 16, th: 16, clusterBits: 12, checkEvery: 1},
		{df: 32, th: 256, clusterBits: 10},
	} {
		for seed := int64(1); seed <= 3; seed++ {
			t.Run(fmt.Sprintf("df=%d th=%d seed=%d", c.df, c.th, seed), func(t *testing.T) {
				testRandomOps(t, c, seed)
			})
		}
	}
}

func testRandomOps(t *testing.T, c canonicalConfig, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	// uniform ids spread over the tree, three clusters each fill one deep range beyond the threshold
	uniform := make([]string, max(2*c.df*c.th, 512))
	for i := range uniform {
		uniform[i] = fmt.Sprintf("id%d", i)
	}
	var clustered []string
	var prefixes []uint64
	for len(prefixes) < 3 {
		prefix := rng.Uint64() >> (64 - c.clusterBits)
		if slices.Contains(prefixes, prefix) {
			continue
		}
		prefixes = append(prefixes, prefix)
		clustered = append(clustered, idsWithPrefix(prefix, c.clusterBits, 4*c.th)...)
	}
	d := newDiff(c.df, c.th).(*diff)
	model := map[string]string{}
	liveUniform, liveClustered := newIdSet(), newIdSet()
	liveOf := func(id string) *idSet {
		if strings.HasPrefix(id, "p") {
			return liveClustered
		}
		return liveUniform
	}
	anyId := func() string {
		if rng.Intn(2) == 0 {
			return clustered[rng.Intn(len(clustered))]
		}
		return uniform[rng.Intn(len(uniform))]
	}
	set := func(batch int) {
		els := make([]Element, 0, batch)
		for i := 0; i < batch; i++ {
			id := anyId()
			head := fmt.Sprintf("h%d", rng.Intn(1<<20))
			if cur, ok := model[id]; ok && rng.Intn(2) == 0 {
				head = cur
			}
			els = append(els, Element{Id: id, Head: head})
			model[id] = head
			liveOf(id).add(id)
		}
		d.Set(els...)
	}
	var maxMerged int
	// removes uniform ids first when uniformFirst, so the clusters drain last and their chains merge
	remove := func(uniformFirst bool) {
		if !uniformFirst && rng.Intn(10) == 0 {
			if id := anyId(); model[id] == "" {
				require.ErrorIs(t, d.RemoveId(id), ErrElementNotFound)
				return
			}
		}
		live := liveClustered
		if len(liveUniform.ids) > 0 && (len(liveClustered.ids) == 0 || uniformFirst || rng.Intn(4) != 0) {
			live = liveUniform
		}
		if len(live.ids) == 0 {
			return
		}
		id := live.pick(rng)
		ranges := len(d.ranges.ranges)
		require.NoError(t, d.RemoveId(id))
		maxMerged = max(maxMerged, (ranges-len(d.ranges.ranges))/c.df)
		delete(model, id)
		live.del(id)
	}
	pool := len(uniform) + len(clustered)
	every := c.checkEvery
	if every == 0 {
		every = pool / 16
	}
	var op int
	check := func(phase string) {
		requireCanonical(t, d, fmt.Sprintf("df=%d th=%d seed=%d phase=%s op=%d", c.df, c.th, seed, phase, op))
	}
	for i, setPercent := range []int{80, 50, 15} {
		phase := fmt.Sprintf("%d(set %d%%)", i, setPercent)
		for j := 0; j < pool; j++ {
			if rng.Intn(100) < setPercent {
				batch := 1
				if rng.Intn(10) == 0 {
					batch = 1 + rng.Intn(16)
				}
				set(batch)
			} else {
				remove(false)
			}
			op++
			if op%every == 0 {
				check(phase)
			}
		}
		check(phase)
		requireElements(t, d, model)
	}
	// a fresh build does not depend on insertion order
	shuffled := d.Elements()
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	fresh := New(c.df, c.th)
	fresh.Set(shuffled...)
	require.Equal(t, fresh.Hash(), d.Hash())

	// full clusters drain last, so every cluster chain merges level by level
	els := make([]Element, 0, len(clustered))
	for _, id := range clustered {
		head := fmt.Sprintf("h%d", rng.Intn(1<<20))
		els = append(els, Element{Id: id, Head: head})
		model[id] = head
		liveClustered.add(id)
	}
	d.Set(els...)
	check("refill")
	for len(liveUniform.ids)+len(liveClustered.ids) > 0 {
		remove(true)
		op++
		if op%every == 0 {
			check("drain")
		}
	}
	check("drain")
	require.Equal(t, 0, d.Len())
	require.Len(t, d.ranges.ranges, 1+c.df)
	require.GreaterOrEqual(t, maxMerged, 2, "no removal merged more than one level")
}

func TestDiff_SetExistingKeepsCount(t *testing.T) {
	d := newDiff(16, 16).(*diff)
	els := make([]Element, 100)
	for i := range els {
		els[i] = Element{Id: fmt.Sprintf("id%d", i), Head: fmt.Sprintf("h%d", i)}
	}
	d.Set(els...)
	for i := 0; i < 5; i++ {
		d.Set(els...)
	}
	res, err := d.Ranges(context.Background(), []Range{{From: 0, To: math.MaxUint64}}, nil)
	require.NoError(t, err)
	require.Equal(t, 100, res[0].Count)
	requireCanonical(t, d, t.Name())
}

func TestDiff_SetExistingAtThreshold(t *testing.T) {
	for _, p := range []struct {
		df, th int
		bits   uint
	}{{16, 16, 4}, {32, 256, 5}} {
		t.Run(fmt.Sprintf("df=%d th=%d", p.df, p.th), func(t *testing.T) {
			// exactly compareThreshold elements in one root bucket: a leaf that must stay a leaf
			ids := idsWithPrefix(0, p.bits, p.th)
			els := make([]Element, len(ids))
			for i, id := range ids {
				els[i] = Element{Id: id, Head: "h"}
			}
			d := newDiff(p.df, p.th).(*diff)
			d.Set(els...)
			leaf := d.ranges.ranges[prefixTuple(0, p.bits)]
			require.False(t, leaf.isDivided)
			hash := d.Hash()

			d.Set(els[0])
			require.Equal(t, hash, d.Hash(), "same head re-set changes the hash")
			require.False(t, leaf.isDivided, "same head re-set divides the leaf")
			requireCanonical(t, d, t.Name())

			d.Set(Element{Id: ids[0], Head: "changed"})
			require.NotEqual(t, hash, d.Hash())
			require.False(t, leaf.isDivided, "head change divides the leaf")
			requireCanonical(t, d, t.Name())
		})
	}
}

func TestDiff_GrowShrinkMergesBack(t *testing.T) {
	d := newDiff(16, 16).(*diff)
	els := make([]Element, 20000)
	for i := range els {
		els[i] = Element{Id: fmt.Sprintf("id%d", i), Head: fmt.Sprintf("h%d", i)}
	}
	d.Set(els...)
	for round := 0; round < 3; round++ {
		d.Set(els...)
	}
	for _, el := range els[1000:] {
		require.NoError(t, d.RemoveId(el.Id))
	}
	fresh := New(16, 16)
	fresh.Set(els[:1000]...)
	require.Equal(t, fresh.Hash(), d.Hash())
	requireCanonical(t, d, t.Name())
}

func TestHashRanges_RemoveMergesAncestors(t *testing.T) {
	// five elements in one level-2 bucket: root -> A -> AA -> leaves, A and AA divided
	ids := idsWithPrefix(0, 4, 5)
	d := newDiff(4, 4).(*diff)
	for _, id := range ids {
		d.Set(Element{Id: id, Head: "h"})
	}
	a := d.ranges.ranges[prefixTuple(0, 2)]
	aa := d.ranges.ranges[prefixTuple(0, 4)]
	require.True(t, a.isDivided)
	require.True(t, aa.isDivided)
	requireCanonical(t, d, t.Name())

	// four left: both A and AA fit the threshold and must be leaves again
	require.NoError(t, d.RemoveId(ids[0]))
	require.False(t, aa.isDivided)
	require.False(t, a.isDivided, "grandparent stays divided after shrinking to the threshold")
	requireCanonical(t, d, t.Name())
}

func TestHashRanges_RemoveLeavesNoOrphans(t *testing.T) {
	// P = root bucket 0 holds C (five elements, all in its bucket CC) and L (one element)
	ccIds := idsWithPrefix(0, 6, 5)
	lId := idsWithPrefix(1, 4, 1)[0]
	pTuple, cTuple, ccTuple := prefixTuple(0, 2), prefixTuple(0, 4), prefixTuple(0, 6)
	d := newDiff(4, 4).(*diff)
	for _, id := range ccIds {
		d.Set(Element{Id: id, Head: "h"})
	}
	d.Set(Element{Id: lId, Head: "h"})
	require.True(t, d.ranges.ranges[pTuple].isDivided)
	require.True(t, d.ranges.ranges[cTuple].isDivided)
	require.True(t, d.ranges.ranges[ccTuple].isDivided)
	requireCanonical(t, d, t.Name())

	// CC drops to the threshold, then P does: every range below P must go
	require.NoError(t, d.RemoveId(ccIds[0]))
	requireCanonical(t, d, t.Name())
	require.NoError(t, d.RemoveId(lId))
	require.False(t, d.ranges.ranges[pTuple].isDivided)
	for tuple := range d.ranges.ranges {
		if tuple != pTuple && tuple.from >= pTuple.from && tuple.to <= pTuple.to {
			t.Fatalf("orphan range %x-%x below leaf %x-%x", tuple.from, tuple.to, pTuple.from, pTuple.to)
		}
	}
	requireCanonical(t, d, t.Name())

	// CC changes while P is a leaf: a request for CC must see the current elements
	d.Set(Element{Id: lId, Head: "h"})
	require.NoError(t, d.RemoveId(ccIds[1]))
	fresh := New(4, 4)
	fresh.Set(d.Elements()...)
	ctx := context.Background()
	want, err := fresh.Ranges(ctx, []Range{{From: ccTuple.from, To: ccTuple.to}}, nil)
	require.NoError(t, err)
	got, err := d.Ranges(ctx, []Range{{From: ccTuple.from, To: ccTuple.to}}, nil)
	require.NoError(t, err)
	require.Equal(t, want, got)
	requireCanonical(t, d, t.Name())
}

func benchElements(n int) []Element {
	els := make([]Element, n)
	for i := range els {
		els[i] = Element{Id: fmt.Sprintf("id%d", i), Head: fmt.Sprintf("head%d", i)}
	}
	return els
}

func BenchmarkDiff_SetBatch(b *testing.B) {
	els := benchElements(10000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New(16, 16).Set(els...)
	}
}

func BenchmarkDiff_SetNew(b *testing.B) {
	d := New(16, 16)
	d.Set(benchElements(10000)...)
	ids := make([]string, b.N)
	for i := range ids {
		ids[i] = fmt.Sprintf("new%d", i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Set(Element{Id: ids[i], Head: "head"})
	}
}

func BenchmarkDiff_SetExistingSameHead(b *testing.B) {
	els := benchElements(10000)
	d := New(16, 16)
	d.Set(els...)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Set(els[i%len(els)])
	}
}

func BenchmarkDiff_SetExistingNewHead(b *testing.B) {
	els := benchElements(10000)
	d := New(16, 16)
	d.Set(els...)
	heads := make([]string, b.N)
	for i := range heads {
		heads[i] = fmt.Sprintf("h%d", i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Set(Element{Id: els[i%len(els)].Id, Head: heads[i]})
	}
}

func BenchmarkDiff_SetRemove(b *testing.B) {
	d := New(16, 16)
	d.Set(benchElements(10000)...)
	ids := make([]string, b.N)
	for i := range ids {
		ids[i] = fmt.Sprintf("new%d", i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Set(Element{Id: ids[i], Head: "head"})
		_ = d.RemoveId(ids[i])
	}
}
