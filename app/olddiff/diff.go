//go:generate mockgen -destination mock_olddiff/mock_olddiff.go github.com/anyproto/any-sync/app/olddiff Diff,Remote
package olddiff

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"math"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/huandu/skiplist"
	"github.com/zeebo/blake3"

	"github.com/anyproto/any-sync/app/ldiff"
)

// New creates precalculated Diff container
//
// divideFactor - means how many hashes you want to ask for once
//
//	it must be 2 or greater
//	normal value usually between 4 and 64
//
// compareThreshold - means the maximum count of elements remote diff will send directly
//
//	if elements under range will be more - remote diff will send only hash
//	it must be 1 or greater
//	normal value between 8 and 64
//
// Less threshold and divideFactor - less traffic but more requests
func New(divideFactor, compareThreshold int) ldiff.Diff {
	return newDiff(divideFactor, compareThreshold)
}

func newDiff(divideFactor, compareThreshold int) ldiff.Diff {
	if divideFactor < 2 {
		divideFactor = 2
	}
	if compareThreshold < 1 {
		compareThreshold = 1
	}
	d := &diff{
		divideFactor:     divideFactor,
		compareThreshold: compareThreshold,
	}
	d.sl = skiplist.New(d)
	d.ranges = newHashRanges(divideFactor, compareThreshold, d.sl)
	d.ranges.dirty[d.ranges.topRange] = struct{}{}
	d.ranges.recalculateHashes()
	return d
}

var hashersPool = &sync.Pool{
	New: func() any {
		return blake3.New()
	},
}

var ErrElementNotFound = errors.New("ldiff: element not found")

type element struct {
	ldiff.Element
	hash uint64
}

// Diff contains elements and can compare it with Remote diff
type diff struct {
	sl               *skiplist.SkipList
	divideFactor     int
	compareThreshold int
	ranges           *hashRanges
	mu               sync.RWMutex
}

// Compare implements skiplist interface
func (d *diff) Compare(lhs, rhs interface{}) int {
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
func (d *diff) CalcScore(key interface{}) float64 {
	return 0
}

// Set adds or update element in container
func (d *diff) Set(elements ...ldiff.Element) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, e := range elements {
		hash := xxhash.Sum64([]byte(e.Id))
		el := &element{Element: e, hash: hash}
		d.sl.Remove(el)
		d.sl.Set(el, nil)
		d.ranges.addElement(hash)
	}
	d.ranges.recalculateHashes()
}

func (d *diff) Ids() (ids []string) {
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

func (d *diff) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sl.Len()
}

func (d *diff) Elements() (elements []ldiff.Element) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	elements = make([]ldiff.Element, 0, d.sl.Len())

	cur := d.sl.Front()
	for cur != nil {
		el := cur.Key().(*element).Element
		elements = append(elements, el)
		cur = cur.Next()
	}
	return
}

func (d *diff) Element(id string) (ldiff.Element, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	el := d.sl.Get(&element{Element: ldiff.Element{Id: id}, hash: xxhash.Sum64([]byte(id))})
	if el == nil {
		return ldiff.Element{}, ErrElementNotFound
	}
	if e, ok := el.Key().(*element); ok {
		return e.Element, nil
	}
	return ldiff.Element{}, ErrElementNotFound
}

func (d *diff) Hash() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return hex.EncodeToString(d.ranges.hash())
}

// RemoveId removes element by id
func (d *diff) RemoveId(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	hash := xxhash.Sum64([]byte(id))
	el := &element{Element: ldiff.Element{
		Id: id,
	}, hash: hash}
	if d.sl.Remove(el) == nil {
		return ErrElementNotFound
	}
	d.ranges.removeElement(hash)
	d.ranges.recalculateHashes()
	return nil
}

func (d *diff) getRange(r ldiff.Range) (rr ldiff.RangeResult) {
	rng := d.ranges.getRange(r.From, r.To)
	// if we have the division for this range
	if rng != nil {
		rr.Hash = rng.hash
		rr.Count = rng.elements
		if !r.Elements && rng.isDivided {
			return
		}
	}

	el := d.sl.Find(&element{hash: r.From})
	rr.Elements = make([]ldiff.Element, 0, d.divideFactor)
	for el != nil && el.Key().(*element).hash <= r.To {
		elem := el.Key().(*element).Element
		el = el.Next()
		rr.Elements = append(rr.Elements, elem)
	}
	rr.Count = len(rr.Elements)
	return
}

// Ranges calculates given ranges and return results
func (d *diff) Ranges(ctx context.Context, ranges []ldiff.Range, resBuf []ldiff.RangeResult) (results []ldiff.RangeResult, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	results = resBuf[:0]
	for _, r := range ranges {
		results = append(results, d.getRange(r))
	}
	return
}

type diffCtx struct {
	newIds, changedIds, removedIds []string

	toSend, prepare []ldiff.Range
	myRes, otherRes []ldiff.RangeResult
}

var errMismatched = errors.New("query and results mismatched")

// Diff makes diff with remote container
func (d *diff) Diff(ctx context.Context, dl ldiff.Remote) (newIds, changedIds, removedIds []string, err error) {
	dctx := &diffCtx{}
	dctx.toSend = append(dctx.toSend, ldiff.Range{
		From: 0,
		To:   math.MaxUint64,
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

func (d *diff) compareResults(dctx *diffCtx, r ldiff.Range, myRes, otherRes ldiff.RangeResult) {
	// both hash equals - do nothing
	if bytes.Equal(myRes.Hash, otherRes.Hash) {
		return
	}

	// other has elements
	if len(otherRes.Elements) == otherRes.Count {
		if len(myRes.Elements) == myRes.Count {
			d.compareElements(dctx, myRes.Elements, otherRes.Elements)
		} else {
			r.Elements = true
			d.compareElements(dctx, d.getRange(r).Elements, otherRes.Elements)
		}
		return
	}
	// request all elements from other, because we don't have enough
	if len(myRes.Elements) == myRes.Count {
		r.Elements = true
		dctx.prepare = append(dctx.prepare, r)
		return
	}
	rangeTuples := genTupleRanges(r.From, r.To, d.divideFactor)
	for _, tuple := range rangeTuples {
		dctx.prepare = append(dctx.prepare, ldiff.Range{From: tuple.from, To: tuple.to})
	}
	return
}

func (d *diff) compareElements(dctx *diffCtx, my, other []ldiff.Element) {
	find := func(list []ldiff.Element, targetEl ldiff.Element) (has, eq bool) {
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
