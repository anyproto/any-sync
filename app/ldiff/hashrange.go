package ldiff

import (
	"math"

	"github.com/huandu/skiplist"
	"github.com/zeebo/blake3"
	"golang.org/x/exp/slices"
)

type hashRange struct {
	from, to  uint64
	parent    *hashRange
	isDivided bool
	elements  int
	level     int
	hash      []byte
}

type rangeTuple struct {
	from, to uint64
}

type hashRanges struct {
	ranges           map[rangeTuple]*hashRange
	topRange         *hashRange
	sl               *skiplist.SkipList
	dirty            map[*hashRange]struct{}
	divideFactor     int
	compareThreshold int
}

func newHashRanges(divideFactor, compareThreshold int, sl *skiplist.SkipList) *hashRanges {
	h := &hashRanges{
		ranges:           make(map[rangeTuple]*hashRange),
		dirty:            make(map[*hashRange]struct{}),
		divideFactor:     divideFactor,
		compareThreshold: compareThreshold,
		sl:               sl,
	}
	h.topRange = &hashRange{
		from:      0,
		to:        math.MaxUint64,
		isDivided: true,
		level:     0,
	}
	h.ranges[rangeTuple{from: 0, to: math.MaxUint64}] = h.topRange
	h.makeBottomRanges(h.topRange)
	return h
}

func (h *hashRanges) hash() []byte {
	return h.topRange.hash
}

func (h *hashRanges) addElement(elHash uint64) {
	rng := h.topRange
	rng.elements++
	for rng.isDivided {
		rng = h.getBottomRange(rng, elHash)
		rng.elements++
	}
	h.dirty[rng] = struct{}{}
	if rng.elements > h.compareThreshold {
		rng.isDivided = true
		h.makeBottomRanges(rng)
	}
	if rng.parent != nil {
		if _, ok := h.dirty[rng.parent]; ok {
			delete(h.dirty, rng.parent)
		}
	}
}

func (h *hashRanges) removeElement(elHash uint64) {
	rng := h.topRange
	rng.elements--
	for rng.isDivided {
		rng = h.getBottomRange(rng, elHash)
		rng.elements--
	}
	parent := rng.parent
	if parent.elements <= h.compareThreshold && parent != h.topRange {
		ranges := genTupleRanges(parent.from, parent.to, h.divideFactor)
		for _, tuple := range ranges {
			child := h.ranges[tuple]
			delete(h.ranges, tuple)
			delete(h.dirty, child)
		}
		parent.isDivided = false
		h.dirty[parent] = struct{}{}
	} else {
		h.dirty[rng] = struct{}{}
	}
}

func (h *hashRanges) recalculateHashes() {
	for len(h.dirty) > 0 {
		var slDirty []*hashRange
		for rng := range h.dirty {
			slDirty = append(slDirty, rng)
		}
		slices.SortFunc(slDirty, func(a, b *hashRange) int {
			if a.level < b.level {
				return -1
			} else if a.level > b.level {
				return 1
			} else {
				return 0
			}
		})
		for _, rng := range slDirty {
			if rng.isDivided {
				rng.hash = h.calcDividedHash(rng)
			} else {
				rng.hash, rng.elements = h.calcElementsHash(rng.from, rng.to)
			}
			delete(h.dirty, rng)
			if rng.parent != nil {
				h.dirty[rng.parent] = struct{}{}
			}
		}
	}
}

func (h *hashRanges) getRange(from, to uint64) *hashRange {
	return h.ranges[rangeTuple{from: from, to: to}]
}

func (h *hashRanges) getBottomRange(rng *hashRange, elHash uint64) *hashRange {
	df := uint64(h.divideFactor)
	perRange := (rng.to - rng.from) / df
	align := ((rng.to-rng.from)%df + 1) % df
	if align == 0 {
		perRange++
	}
	bucket := (elHash - rng.from) / perRange
	tuple := rangeTuple{from: rng.from + bucket*perRange, to: rng.from - 1 + (bucket+1)*perRange}
	if bucket == df-1 {
		tuple.to += align
	}
	return h.ranges[tuple]
}

func (h *hashRanges) makeBottomRanges(rng *hashRange) {
	ranges := genTupleRanges(rng.from, rng.to, h.divideFactor)
	for _, tuple := range ranges {
		newRange := h.makeRange(tuple, rng)
		h.ranges[tuple] = newRange
		if newRange.elements > h.compareThreshold {
			if _, ok := h.dirty[rng]; ok {
				delete(h.dirty, rng)
			}
			h.dirty[newRange] = struct{}{}
			newRange.isDivided = true
			h.makeBottomRanges(newRange)
		}
	}
}

func (h *hashRanges) makeRange(tuple rangeTuple, parent *hashRange) *hashRange {
	newRange := &hashRange{
		from:   tuple.from,
		to:     tuple.to,
		parent: parent,
	}
	hash, els := h.calcElementsHash(tuple.from, tuple.to)
	newRange.hash = hash
	newRange.level = parent.level + 1
	newRange.elements = els
	return newRange
}

func (h *hashRanges) calcDividedHash(rng *hashRange) (hash []byte) {
	hasher := hashersPool.Get().(*blake3.Hasher)
	defer hashersPool.Put(hasher)
	hasher.Reset()
	ranges := genTupleRanges(rng.from, rng.to, h.divideFactor)
	for _, tuple := range ranges {
		child := h.ranges[tuple]
		hasher.Write(child.hash)
	}
	hash = hasher.Sum(nil)
	return
}

func genTupleRanges(from, to uint64, divideFactor int) (prepare []rangeTuple) {
	df := uint64(divideFactor)
	perRange := (to - from) / df
	align := ((to-from)%df + 1) % df
	if align == 0 {
		perRange++
	}
	var j = from
	for i := 0; i < divideFactor; i++ {
		if i == divideFactor-1 {
			perRange += align
		}
		prepare = append(prepare, rangeTuple{from: j, to: j + perRange - 1})
		j += perRange
	}
	return
}

func (h *hashRanges) calcElementsHash(from, to uint64) (hash []byte, els int) {
	hasher := hashersPool.Get().(*blake3.Hasher)
	defer hashersPool.Put(hasher)
	hasher.Reset()

	el := h.sl.Find(&element{hash: from})
	for el != nil && el.Key().(*element).hash <= to {
		elem := el.Key().(*element).Element
		el = el.Next()

		hasher.WriteString(elem.Id)
		hasher.WriteString(elem.Head)
		els++
	}
	if els != 0 {
		hash = hasher.Sum(nil)
	}
	return
}
