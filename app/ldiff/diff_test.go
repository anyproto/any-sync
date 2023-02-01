package ldiff

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/mgo.v2/bson"
	"math"
	"sort"
	"testing"
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
		r := Range{From: 0, To: math.MaxUint64, Limit: 10}
		res := d.getRange(r)
		assert.NotNil(t, res.Hash)
		assert.Len(t, res.Elements, 10)
	})
	t.Run("hash", func(t *testing.T) {
		r := Range{From: 0, To: math.MaxUint64, Limit: 9}
		res := d.getRange(r)
		t.Log(len(res.Elements))
		assert.NotNil(t, res.Hash)
		assert.Nil(t, res.Elements)
	})
}

func TestDiff_Diff(t *testing.T) {
	ctx := context.Background()
	t.Run("basic", func(t *testing.T) {
		d1 := New(16, 16)
		d2 := New(16, 16)
		for i := 0; i < 1000; i++ {
			id := fmt.Sprint(i)
			head := bson.NewObjectId().Hex()
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
				Head: bson.NewObjectId().Hex(),
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
				Head: bson.NewObjectId().Hex(),
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
		head := bson.NewObjectId().Hex()
		d.Set(Element{
			Id:   id,
			Head: head,
		})
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	var resBuf []RangeResult
	var ranges = []Range{{From: 0, To: math.MaxUint64, Limit: 10}}
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
