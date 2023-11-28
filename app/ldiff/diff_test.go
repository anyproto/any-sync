package ldiff

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiff_fillRange(t *testing.T) {
	d := New(4, 4).(*diff)
	for i := 0; i < 10; i++ {
		el := Element{
			Id:   fmt.Sprint(i),
			Head: fmt.Sprint("h", i),
		}
		d.Set(el)
	}
	t.Log(d.sl.Len())

	t.Run("elements", func(t *testing.T) {
		r := Range{From: 0, To: math.MaxUint64}
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
			d1.Set(Element{
				Id:   id,
				Head: head,
			})
			d2.Set(Element{
				Id:   id,
				Head: head,
			})
		}

		newIds, changedIds, removedIds, err := d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 0)
		assert.Len(t, changedIds, 0)
		assert.Len(t, removedIds, 0)

		d2.Set(Element{
			Id:   "newD1",
			Head: "newD1",
		})
		d2.Set(Element{
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
			d1.Set(Element{
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
			d2.Set(Element{
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
			d2.Set(Element{
				Id:   id,
				Head: head,
			})
		}

		res, err := d1.Ranges(
			context.Background(),
			[]Range{{From: 0, To: math.MaxUint64, Elements: true}},
			nil)
		require.NoError(t, err)
		require.Len(t, res, 1)
		for i, el := range res[0].Elements {
			if i < length/2 {
				continue
			}
			id := el.Id
			head := el.Head
			d2.Set(Element{
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
		for i := 0; i < 10; i++ {
			d2.Set(Element{
				Id:   fmt.Sprint(i),
				Head: uuid.NewString(),
			})
		}

		newIds, changedIds, removedIds, err := d1.Diff(ctx, d2)
		require.NoError(t, err)
		assert.Len(t, newIds, 10)
		assert.Len(t, changedIds, 0)
		assert.Len(t, removedIds, 0)
	})
	t.Run("context cancel", func(t *testing.T) {
		d1 := New(4, 4)
		d2 := New(4, 4)
		for i := 0; i < 10; i++ {
			d2.Set(Element{
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
		d.Set(Element{
			Id:   id,
			Head: head,
		})
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	var resBuf []RangeResult
	var ranges = []Range{{From: 0, To: math.MaxUint64}}
	for i := 0; i < b.N; i++ {
		d.Ranges(ctx, ranges, resBuf)
		resBuf = resBuf[:0]
	}
}

func TestDiff_Hash(t *testing.T) {
	d := New(16, 16)
	h1 := d.Hash()
	assert.NotEmpty(t, h1)
	d.Set(Element{Id: "1"})
	h2 := d.Hash()
	assert.NotEmpty(t, h2)
	assert.NotEqual(t, h1, h2)
}

func TestDiff_Element(t *testing.T) {
	d := New(16, 16)
	for i := 0; i < 10; i++ {
		d.Set(Element{Id: fmt.Sprint("id", i), Head: fmt.Sprint("head", i)})
	}
	_, err := d.Element("not found")
	assert.Equal(t, ErrElementNotFound, err)

	el, err := d.Element("id5")
	require.NoError(t, err)
	assert.Equal(t, "head5", el.Head)

	d.Set(Element{"id5", "otherHead"})
	el, err = d.Element("id5")
	require.NoError(t, err)
	assert.Equal(t, "otherHead", el.Head)
}

func TestDiff_Ids(t *testing.T) {
	d := New(16, 16)
	var ids []string
	for i := 0; i < 10; i++ {
		id := fmt.Sprint("id", i)
		d.Set(Element{Id: id, Head: fmt.Sprint("head", i)})
		ids = append(ids, id)
	}
	gotIds := d.Ids()
	sort.Strings(gotIds)
	assert.Equal(t, ids, gotIds)
	assert.Equal(t, len(ids), d.Len())
}

func TestDiff_Elements(t *testing.T) {
	d := New(16, 16)
	var els []Element
	for i := 0; i < 10; i++ {
		id := fmt.Sprint("id", i)
		el := Element{Id: id, Head: fmt.Sprint("head", i)}
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
		var els []Element
		for i := 0; i < length; i++ {
			if i < length/20 {
				continue
			}
			els = append(els, Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		els = els[:0]
		for i := 0; i < length/20; i++ {
			els = append(els, Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		return d.Hash()
	}
	addOnce := func() string {
		d := New(divideFactor, compareThreshold)
		var els []Element
		for i := 0; i < length; i++ {
			els = append(els, Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		return d.Hash()
	}
	addRemove := func() string {
		d := New(divideFactor, compareThreshold)
		var els []Element
		for i := 0; i < length; i++ {
			els = append(els, Element{
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
			els = append(els, Element{
				Id:   fmt.Sprint(i),
				Head: fmt.Sprint("h", i),
			})
		}
		d.Set(els...)
		return d.Hash()
	}
	require.Equal(t, addTwice(), addOnce(), addRemove())
}
