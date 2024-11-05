package objecttree

import (
	"math"

	"github.com/anyproto/any-sync/util/slice"
)

// clearPossibleRoots force removes any snapshots which can further be deemed as roots
func (t *Tree) clearPossibleRoots() {
	t.possibleRoots = t.possibleRoots[:0]
}

// checkRoot checks if a change can be a new root for the tree
// it returns total changes which were discovered during dfsPrev from heads
func (t *Tree) checkRoot(change *Change) (total int) {
	t.stackBuf = t.stackBuf[:0]
	stack := t.stackBuf

	// starting with heads
	for _, h := range t.headIds {
		stack = append(stack, t.attached[h])
	}

	t.dfsPrev(
		stack,
		[]string{change.Id},
		func(ch *Change) bool {
			total += 1
			return true
		},
		func(changes []*Change) {
			if t.root.visited {
				total = -1
			}
		},
	)

	return
}

// makeRootAndRemove removes all changes before start and makes start the root
func (t *Tree) makeRootAndRemove(start *Change) {
	t.stackBuf = t.stackBuf[:0]
	stack := t.stackBuf
	for _, prev := range start.PreviousIds {
		stack = append(stack, t.attached[prev])
	}

	t.dfsPrev(
		stack,
		[]string{},
		func(ch *Change) bool {
			return true
		},
		func(changes []*Change) {
			for _, ch := range changes {
				delete(t.attached, ch.Id)
			}
		},
	)

	// removing unattached because they may refer to previous root
	t.unAttached = make(map[string]*Change)
	t.root = start
}

// reduceTree tries to reduce the tree to one of possible tree roots
func (t *Tree) reduceTree() (res bool) {
	if len(t.possibleRoots) == 0 {
		return
	}
	var (
		minRoot  *Change
		minTotal = math.MaxInt
	)
	for _, r := range t.possibleRoots {
		// if this is snapshot and next is also snapshot, then we don't need to take this one into account
		if len(r.Next) == 1 && r.Next[0].IsSnapshot {
			r.visited = true
		}
	}
	t.possibleRoots = slice.DiscardFromSlice(t.possibleRoots, func(change *Change) bool {
		if change.visited {
			change.visited = false
			return true
		}
		return false
	})
	// TODO: this can be further optimized by iterating the tree and checking the roots from top to bottom

	// checking if we can reduce tree to other root
	for _, root := range t.possibleRoots {
		totalChanges := t.checkRoot(root)
		// we prefer new root with min amount of total changes
		if totalChanges != -1 && totalChanges < minTotal {
			minRoot = root
			minTotal = totalChanges
		}
	}

	t.clearPossibleRoots()
	if minRoot == nil {
		return
	}

	t.makeRootAndRemove(minRoot)
	res = true
	return
}
