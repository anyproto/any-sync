package olddiff

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/app/ldiff"
)

func TestDiff_fillRange(t *testing.T) {
	d := New(4, 4).(*diff)
	for i := 0; i < 10; i++ {
		el := ldiff.Element{
			Id:   fmt.Sprint(i),
			Head: fmt.Sprint("h", i),
		}
		d.Set(el)
	}
	t.Log(d.sl.Len())

	t.Run("elements", func(t *testing.T) {
		r := ldiff.Range{From: 0, To: math.MaxUint64}
		res := d.getRange(r)
		assert.NotNil(t, res.Hash)
		assert.Equal(t, res.Count, 10)
	})
}

func TestDiff_Diff(t *testing.T) {
	ctx := context.Background()
	t.Run("basic", func(t *testing.T) {
		d1 := New(16, 16)
		d2 := New(16, 16)
		for i := 0; i < 1000; i++ {
			id := fmt.Sprint(i)
			head := uuid.NewString()
			d1.Set(ldiff.Element{
				Id:   id,
				Head: head,
			})
			d2.Set(ldiff.Element{
				Id:   id,
				Head: head,
			})
		}

		newIds, changedIds, removedIds, err := d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 0)
		assert.Len(t, changedIds, 0)
		assert.Len(t, removedIds, 0)

		d2.Set(ldiff.Element{
			Id:   "newD1",
			Head: "newD1",
		})
		d2.Set(ldiff.Element{
			Id:   "1",
			Head: "changed",
		})
		require.NoError(t, d2.RemoveId("0"))

		newIds, changedIds, removedIds, err = d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 1)
		assert.Len(t, changedIds, 1)
		assert.Len(t, removedIds, 1)
	})
	t.Run("complex", func(t *testing.T) {
		d1 := New(16, 128)
		d2 := New(16, 128)
		length := 10000
		for i := 0; i < length; i++ {
			id := fmt.Sprint(i)
			head := uuid.NewString()
			d1.Set(ldiff.Element{
				Id:   id,
				Head: head,
			})
		}

		newIds, changedIds, removedIds, err := d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 0)
		assert.Len(t, changedIds, 0)
		assert.Len(t, removedIds, length)

		for i := 0; i < length; i++ {
			id := fmt.Sprint(i)
			head := uuid.NewString()
			d2.Set(ldiff.Element{
				Id:   id,
				Head: head,
			})
		}

		newIds, changedIds, removedIds, err = d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 0)
		assert.Len(t, changedIds, length)
		assert.Len(t, removedIds, 0)

		for i := 0; i < length; i++ {
			id := fmt.Sprint(i)
			head := uuid.NewString()
			d2.Set(ldiff.Element{
				Id:   id,
				Head: head,
			})
		}

		res, err := d1.Ranges(
			context.Background(),
			[]ldiff.Range{{From: 0, To: math.MaxUint64, Elements: true}},
			nil)
		require.NoError(t, err)
		require.Len(t, res, 1)
		for i, el := range res[0].Elements {
			if i < length/2 {
				continue
			}
			id := el.Id
			head := el.Head
			d2.Set(ldiff.Element{
				Id:   id,
				Head: head,
			})
		}

		newIds, changedIds, removedIds, err = d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 0)
		assert.Len(t, changedIds, length/2)
		assert.Len(t, removedIds, 0)
	})
	t.Run("empty", func(t *testing.T) {
		d1 := New(16, 16)
		d2 := New(16, 16)
		newIds, changedIds, removedIds, err := d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 0)
		assert.Len(t, changedIds, 0)
		assert.Len(t, removedIds, 0)
	})
	t.Run("one empty", func(t *testing.T) {
		d1 := New(4, 4)
		d2 := New(4, 4)
		length := 10000
		for i := 0; i < length; i++ {
			d2.Set(ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: uuid.NewString(),
			})
		}
		newIds, changedIds, removedIds, err := d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, length)
		assert.Len(t, changedIds, 0)
		assert.Len(t, removedIds, 0)
	})
	t.Run("not intersecting", func(t *testing.T) {
		d1 := New(16, 16)
		d2 := New(16, 16)
		length := 10000
		for i := 0; i < length; i++ {
			d1.Set(ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: uuid.NewString(),
			})
		}
		for i := length; i < length*2; i++ {
			d2.Set(ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: uuid.NewString(),
			})
		}
		newIds, changedIds, removedIds, err := d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, length)
		assert.Len(t, changedIds, 0)
		assert.Len(t, removedIds, length)
	})
	t.Run("context cancel", func(t *testing.T) {
		d1 := New(4, 4)
		d2 := New(4, 4)
		for i := 0; i < 10; i++ {
			d2.Set(ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: uuid.NewString(),
			})
		}
		var cancel func()
		ctx, cancel = context.WithCancel(ctx)
		cancel()
		_, _, _, err := d1.Diff(ctx, d2)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func BenchmarkDiff_Ranges(b *testing.B) {
	d := New(16, 16)
	for i := 0; i < 10000; i++ {
		id := fmt.Sprint(i)
		head := uuid.NewString()
		d.Set(ldiff.Element{
			Id:   id,
			Head: head,
		})
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	var resBuf []ldiff.RangeResult
	var ranges = []ldiff.Range{{From: 0, To: math.MaxUint64}}
	for i := 0; i < b.N; i++ {
		d.Ranges(ctx, ranges, resBuf)
		resBuf = resBuf[:0]
	}
}

func TestDiff_Hash(t *testing.T) {
	d := New(16, 16)
	h1 := d.Hash()
	assert.NotEmpty(t, h1)
	d.Set(ldiff.Element{Id: "1"})
	h2 := d.Hash()
	assert.NotEmpty(t, h2)
	assert.NotEqual(t, h1, h2)
}

func TestDiff_Element(t *testing.T) {
	d := New(16, 16)
	for i := 0; i < 10; i++ {
		d.Set(ldiff.Element{Id: fmt.Sprint("id", i), Head: fmt.Sprint("head", i)})
	}
	_, err := d.Element("not found")
	assert.Equal(t, ErrElementNotFound, err)

	el, err := d.Element("id5")
	require.NoError(t, err)
	assert.Equal(t, "head5", el.Head)

	d.Set(ldiff.Element{"id5", "otherHead"})
	el, err = d.Element("id5")
	require.NoError(t, err)
	assert.Equal(t, "otherHead", el.Head)
}

func TestDiff_Ids(t *testing.T) {
	d := New(16, 16)
	var ids []string
	for i := 0; i < 10; i++ {
		id := fmt.Sprint("id", i)
		d.Set(ldiff.Element{Id: id, Head: fmt.Sprint("head", i)})
		ids = append(ids, id)
	}
	gotIds := d.Ids()
	sort.Strings(gotIds)
	assert.Equal(t, ids, gotIds)
	assert.Equal(t, len(ids), d.Len())
}

func TestDiff_Elements(t *testing.T) {
	d := New(16, 16)
	var els []ldiff.Element
	for i := 0; i < 10; i++ {
		id := fmt.Sprint("id", i)
		el := ldiff.Element{Id: id, Head: fmt.Sprint("head", i)}
		d.Set(el)
		els = append(els, el)
	}
	gotEls := d.Elements()
	sort.Slice(gotEls, func(i, j int) bool {
		return gotEls[i].Id < gotEls[j].Id
	})
	assert.Equal(t, els, gotEls)
}

func TestRangesAddRemove(t *testing.T) {
	length := 10000
	divideFactor := 4
	compareThreshold := 4
	addTwice := func() string {
		d := New(divideFactor, compareThreshold)
		var els []ldiff.Element
		for i := 0; i < length; i++ {
			if i < length/20 {
				continue
			}
			els = append(els, ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		els = els[:0]
		for i := 0; i < length/20; i++ {
			els = append(els, ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		return d.Hash()
	}
	addOnce := func() string {
		d := New(divideFactor, compareThreshold)
		var els []ldiff.Element
		for i := 0; i < length; i++ {
			els = append(els, ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		return d.Hash()
	}
	addRemove := func() string {
		d := New(divideFactor, compareThreshold)
		var els []ldiff.Element
		for i := 0; i < length; i++ {
			els = append(els, ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		for i := 0; i < length/20; i++ {
			err := d.RemoveId(fmt.Sprint(i))
			require.NoError(t, err)
		}
		els = els[:0]
		for i := 0; i < length/20; i++ {
			els = append(els, ldiff.Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		return d.Hash()
	}
	require.Equal(t, addTwice(), addOnce(), addRemove())
}

func printBestParams() {
	numTests := 10
	length := 100000
	calcParams := func(divideFactor, compareThreshold, length int) (total, maxLevel, avgLevel, zeroEls int) {
		d := New(divideFactor, compareThreshold).(*diff)
		var els []ldiff.Element
		for i := 0; i < length; i++ {
			els = append(els, ldiff.Element{
				Id:   uuid.NewString(),
				Head: uuid.NewString(),
			})
		}
		d.Set(els...)
		for _, rng := range d.ranges.ranges {
			if rng.elements == 0 {
				zeroEls++
			}
			if rng.level > maxLevel {
				maxLevel = rng.level
			}
			avgLevel += rng.level
		}
		total = len(d.ranges.ranges)
		avgLevel = avgLevel / total
		return
	}
	type result struct {
		divFactor, compThreshold, numRanges, maxLevel, avgLevel, zeroEls int
	}
	sf := func(i, j result) int {
		if i.numRanges < j.numRanges {
			return -1
		} else if i.numRanges == j.numRanges {
			return 0
		} else {
			return 1
		}
	}
	var results []result
	for divFactor := 0; divFactor < 6; divFactor++ {
		df := 1 << divFactor
		for compThreshold := 0; compThreshold < 10; compThreshold++ {
			ct := 1 << compThreshold
			fmt.Println("starting, df:", df, "ct:", ct)
			var rngs []result
			for i := 0; i < numTests; i++ {
				total, maxLevel, avgLevel, zeroEls := calcParams(df, ct, length)
				rngs = append(rngs, result{
					divFactor:     df,
					compThreshold: ct,
					numRanges:     total,
					maxLevel:      maxLevel,
					avgLevel:      avgLevel,
					zeroEls:       zeroEls,
				})
			}
			slices.SortFunc(rngs, sf)
			ranges := rngs[len(rngs)/2]
			results = append(results, ranges)
		}
	}
	slices.SortFunc(results, sf)
	fmt.Println(results)
	// 100000 - [{16 512 273 2 1 0} {4 512 341 4 3 0} {2 512 511 8 7 0} {1 512 511 8 7 0}
	// {8 256 585 3 2 0} {8 512 585 3 2 0} {1 256 1023 9 8 0} {2 256 1023 9 8 0}
	// {32 256 1057 2 1 0} {32 512 1057 2 1 0} {32 128 1089 3 1 0} {4 256 1365 5 4 0}
	// {4 128 1369 6 4 0} {2 128 2049 11 9 0} {1 128 2049 11 9 0} {1 64 4157 12 10 0}
	// {2 64 4159 12 10 0} {16 128 4369 3 2 0} {16 64 4369 3 2 0} {16 256 4369 3 2 0}
	// {8 64 4681 4 3 0} {8 128 4681 4 3 0} {4 64 5461 6 5 0} {4 32 6389 7 5 0}
	// {8 32 6505 5 4 17} {16 32 8033 4 3 374} {2 32 8619 13 11 0} {1 32 8621 13 11 0}
	// {2 16 17837 15 12 0} {1 16 17847 15 12 0} {4 16 21081 8 6 22} {32 64 33825 3 2 1578}
	// {32 32 33825 3 2 1559} {32 16 33825 3 2 1518} {8 16 35881 5 4 1313} {16 16 66737 4 3 13022}]
	// 1000000 - [{8 256 11753 5 4 0}]
	// 1000000 - [{16 128 69905 4 3 0}]
	// 1000000 - [{32 256 33825 3 2 0}]
}
