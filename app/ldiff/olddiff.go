package ldiff

import (
	"bytes"
	"context"
	"encoding/hex"
	"math"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash"
	"github.com/huandu/skiplist"
	"github.com/zeebo/blake3"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type olddiff struct {
	sl               *skiplist.SkipList
	divideFactor     int
	compareThreshold int
	hashIsDirty      atomic.Bool
	hash             []byte
	mu               *sync.RWMutex
}

func newOldDiff(divideFactor, compareThreshold int, sl *skiplist.SkipList, mu *sync.RWMutex) *olddiff {
	if divideFactor < 2 {
		divideFactor = 2
	}
	if compareThreshold < 1 {
		compareThreshold = 1
	}
	d := &olddiff{
		divideFactor:     divideFactor,
		compareThreshold: compareThreshold,
		sl:               sl,
		mu:               mu,
	}
	return d
}

// Compare implements skiplist interface
func (d *olddiff) Compare(lhs, rhs interface{}) int {
	lhe := lhs.(*element)
	rhe := rhs.(*element)
	if lhe.Id == rhe.Id {
		return 0
	}
	if lhe.hash > rhe.hash {
		return 1
	} else if lhe.hash < rhe.hash {
		return -1
	}
	if lhe.Id > rhe.Id {
		return 1
	} else {
		return -1
	}
}

// CalcScore implements skiplist interface
func (d *olddiff) CalcScore(key interface{}) float64 {
	return 0
}

// Set adds or update element in container
func (d *olddiff) Set(elements ...Element) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hashIsDirty.Store(true)
	for _, e := range elements {
		el := &element{Element: e, hash: xxhash.Sum64([]byte(e.Id))}
		d.sl.Remove(el)
		d.sl.Set(el, nil)
	}
}

func (d *olddiff) Ids() (ids []string) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ids = make([]string, 0, d.sl.Len())

	cur := d.sl.Front()
	for cur != nil {
		el := cur.Key().(*element).Element
		ids = append(ids, el.Id)
		cur = cur.Next()
	}
	return
}

func (d *olddiff) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sl.Len()
}

func (d *olddiff) Elements() (elements []Element) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	elements = make([]Element, 0, d.sl.Len())

	cur := d.sl.Front()
	for cur != nil {
		el := cur.Key().(*element).Element
		elements = append(elements, el)
		cur = cur.Next()
	}
	return
}

func (d *olddiff) Element(id string) (Element, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	el := d.sl.Get(&element{Element: Element{Id: id}, hash: xxhash.Sum64([]byte(id))})
	if el == nil {
		return Element{}, ErrElementNotFound
	}
	if e, ok := el.Key().(*element); ok {
		return e.Element, nil
	}
	return Element{}, ErrElementNotFound
}

func (d *olddiff) Hash() string {
	return hex.EncodeToString(d.caclHash())
}

func (d *olddiff) caclHash() []byte {
	if d.hashIsDirty.Load() {
		// this can be a lengthy operation
		d.mu.RLock()
		res := d.getRange(Range{To: math.MaxUint64})
		d.mu.RUnlock()
		// saving it under write lock
		d.mu.Lock()
		d.hashIsDirty.Store(false)
		d.hash = res.Hash
		d.mu.Unlock()
		return res.Hash
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hash
}

// RemoveId removes element by id
func (d *olddiff) RemoveId(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hashIsDirty.Store(true)
	el := &element{Element: Element{
		Id: id,
	}, hash: xxhash.Sum64([]byte(id))}
	if d.sl.Remove(el) == nil {
		return ErrElementNotFound
	}
	return nil
}

func (d *olddiff) DiffType() spacesyncproto.DiffType {
	return spacesyncproto.DiffType_Initial
}

func (d *olddiff) getRange(r Range) (rr RangeResult) {
	hasher := hashersPool.Get().(*blake3.Hasher)
	defer hashersPool.Put(hasher)
	hasher.Reset()

	el := d.sl.Find(&element{hash: r.From})
	rr.Elements = make([]Element, 0, r.Limit)
	var overfill bool
	for el != nil && el.Key().(*element).hash <= r.To {
		elem := el.Key().(*element).Element
		el = el.Next()

		hasher.WriteString(elem.Id)
		hasher.WriteString(elem.Head)
		rr.Count++
		if !overfill {
			if len(rr.Elements) < r.Limit {
				rr.Elements = append(rr.Elements, elem)
			}
			if len(rr.Elements) == r.Limit && el != nil {
				overfill = true
			}
		}
	}
	if overfill {
		rr.Elements = nil
	}
	rr.Hash = hasher.Sum(nil)
	return
}

// Ranges calculates given ranges and return results
func (d *olddiff) Ranges(ctx context.Context, ranges []Range, resBuf []RangeResult) (results []RangeResult, err error) {
	if len(ranges) == 1 && d.isFullRange(ranges[0]) {
		return []RangeResult{{
			Hash:  d.caclHash(),
			Count: 1,
		}}, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	results = resBuf[:0]
	for _, r := range ranges {
		results = append(results, d.getRange(r))
	}
	return
}

func (d *olddiff) isFullRange(r Range) bool {
	return r.From == 0 && r.To == math.MaxUint64
}

// Diff makes diff with remote container
func (d *olddiff) Diff(ctx context.Context, dl Remote) (newIds, changedIds, removedIds []string, err error) {
	dctx := &diffCtx{}
	dctx.toSend = append(dctx.toSend, Range{
		From:  0,
		To:    math.MaxUint64,
		Limit: d.compareThreshold,
	})
	for len(dctx.toSend) > 0 {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		default:
		}
		if dctx.otherRes, err = dl.Ranges(ctx, dctx.toSend, dctx.otherRes); err != nil {
			return
		}
		if dctx.myRes, err = d.Ranges(ctx, dctx.toSend, dctx.myRes); err != nil {
			return
		}
		if len(dctx.otherRes) != len(dctx.toSend) || len(dctx.myRes) != len(dctx.toSend) {
			err = errMismatched
			return
		}
		for i, r := range dctx.toSend {
			d.compareResults(dctx, r, dctx.myRes[i], dctx.otherRes[i])
		}
		dctx.toSend, dctx.prepare = dctx.prepare, dctx.toSend
		dctx.prepare = dctx.prepare[:0]
	}
	return dctx.newIds, dctx.changedIds, dctx.removedIds, nil
}

func (d *olddiff) markHashDirty() {
	d.hashIsDirty.Store(true)
}

func (d *olddiff) compareResults(dctx *diffCtx, r Range, myRes, otherRes RangeResult) {
	// both hash equals - do nothing
	if bytes.Equal(myRes.Hash, otherRes.Hash) {
		return
	}

	// both has elements
	if len(myRes.Elements) == myRes.Count && len(otherRes.Elements) == otherRes.Count {
		d.compareElements(dctx, myRes.Elements, otherRes.Elements)
		return
	}

	// make more queries
	divideFactor := uint64(d.divideFactor)
	perRange := (r.To - r.From) / divideFactor
	align := ((r.To-r.From)%divideFactor + 1) % divideFactor
	if align == 0 {
		perRange += 1
	}
	var j = r.From
	for i := 0; i < d.divideFactor; i++ {
		if i == d.divideFactor-1 {
			perRange += align
		}
		dctx.prepare = append(dctx.prepare, Range{From: j, To: j + perRange - 1, Limit: r.Limit})
		j += perRange
	}
	return
}

func (d *olddiff) compareElements(dctx *diffCtx, my, other []Element) {
	find := func(list []Element, targetEl Element) (has, eq bool) {
		for _, el := range list {
			if el.Id == targetEl.Id {
				return true, el.Head == targetEl.Head
			}
		}
		return false, false
	}

	for _, el := range my {
		has, eq := find(other, el)
		if !has {
			dctx.removedIds = append(dctx.removedIds, el.Id)
			continue
		} else {
			if !eq {
				dctx.changedIds = append(dctx.changedIds, el.Id)
			}
		}
	}

	for _, el := range other {
		if has, _ := find(my, el); !has {
			dctx.newIds = append(dctx.newIds, el.Id)
		}
	}
}
