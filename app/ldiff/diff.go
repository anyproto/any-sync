// Package ldiff provides a container of elements with fixed id and changeable content.
// Diff can calculate the difference with another diff container (you can make it remote) with minimum hops and traffic.
//
//go:generate mockgen -destination mock_ldiff/mock_ldiff.go github.com/anyproto/any-sync/app/ldiff Diff,Remote
package ldiff

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
)

// Diff contains elements and can compare it with Remote diff
type Diff interface {
	Remote
	// Set adds or update elements in container
	Set(elements ...Element)
	// RemoveId removes element by id
	RemoveId(id string) error
	// Diff makes diff with remote container
	Diff(ctx context.Context, dl Remote) (newIds, changedIds, removedIds []string, err error)
	// Elements retrieves all elements in the Diff
	Elements() []Element
	// Element returns an element by id
	Element(id string) (Element, error)
	// Ids retrieves ids of all elements in the Diff
	Ids() []string
	// Hash returns hash of all elements in the diff
	Hash() string
	// Len returns count of elements in the diff
	Len() int
}

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
func New(divideFactor, compareThreshold int) Diff {
	return newDiff(divideFactor, compareThreshold)
}

func newDiff(divideFactor, compareThreshold int) Diff {
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

// Element of data
type Element struct {
	Id   string
	Head string
}

// Range request to get RangeResult
type Range struct {
	From, To uint64
	Elements bool
	Limit    int
}

// RangeResult response for Range
type RangeResult struct {
	Hash     []byte
	Elements []Element
	Count    int
}

type element struct {
	Element
	hash uint64
}

// Remote interface for using in the Diff
type Remote interface {
	// Ranges calculates given ranges and return results
	Ranges(ctx context.Context, ranges []Range, resBuf []RangeResult) (results []RangeResult, err error)
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
func (d *diff) Set(elements ...Element) {
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

func (d *diff) Elements() (elements []Element) {
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

func (d *diff) Element(id string) (Element, error) {
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
	el := &element{Element: Element{
		Id: id,
	}, hash: hash}
	if d.sl.Remove(el) == nil {
		return ErrElementNotFound
	}
	d.ranges.removeElement(hash)
	d.ranges.recalculateHashes()
	return nil
}

func (d *diff) getRange(r Range) (rr RangeResult) {
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
	rr.Elements = make([]Element, 0, d.divideFactor)
	for el != nil && el.Key().(*element).hash <= r.To {
		elem := el.Key().(*element).Element
		el = el.Next()
		rr.Elements = append(rr.Elements, elem)
	}
	rr.Count = len(rr.Elements)
	return
}

// Ranges calculates given ranges and return results
func (d *diff) Ranges(ctx context.Context, ranges []Range, resBuf []RangeResult) (results []RangeResult, err error) {
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

	toSend, prepare []Range
	myRes, otherRes []RangeResult
}

var errMismatched = errors.New("query and results mismatched")

// Diff makes diff with remote container
func (d *diff) Diff(ctx context.Context, dl Remote) (newIds, changedIds, removedIds []string, err error) {
	dctx := &diffCtx{}
	dctx.toSend = append(dctx.toSend, Range{
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

func (d *diff) compareResults(dctx *diffCtx, r Range, myRes, otherRes RangeResult) {
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
		dctx.prepare = append(dctx.prepare, Range{From: tuple.from, To: tuple.to})
	}
	return
}

func (d *diff) compareElements(dctx *diffCtx, my, other []Element) {
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
