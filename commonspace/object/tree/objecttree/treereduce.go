package objecttree

import (
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

// clearPossibleRoots force removes any snapshots which can further be deemed as roots
func (t *Tree) clearPossibleRoots() {
	t.possibleRoots = t.possibleRoots[:0]
}

// makeRootAndRemove removes all changes before start and makes start the root
func (t *Tree) makeRootAndRemove(start *Change) {
	if start.Id == t.root.Id {
		return
	}
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
			// TODO Although changes are removed from the attached set, they are all still in memory.
			// TODO If prev changes are still in memory, looks like we don't need to load changes for reduced tree (if we require them) from the store.
			for _, ch := range changes {
				delete(t.attached, ch.Id)
			}
		},
	)

	// removing unattached because they may refer to previous root
	t.unAttached = make(map[string]*Change)
	t.root = start
}

// reduceTree tries to reduce the tree to one of possible tree roots. If we need to add change to the truncated tree,
// we have to rebuild tree starting from the snapshot of the added change.
/* Let's see an example. For clarity, let's look only on snapshots, just imagine that after each snapshot we have a lot of changes.
Let's reduce this tree with heads [sn 5, sn 3, sn 4]:
          ┌────┐
          │sn 1│
          └──┬─┘
             │
          ┌──▼─┐
          │sn 2├──────┐
          └──┬─┘      │
             │        │
          ┌──▼─┐   ┌──▼─┐
      ┌───│sn 3│   │sn 4│
      │   └────┘   └────┘
   ┌──▼─┐
   │sn 5│
   └────┘

The snapshots path from the first head (sn 5 here) to the root is: sn 5 -> sn 3 -> sn 2 -> sn 1
For head sn 3 it's: sn 3 -> sn 2 -> sn 1
For head sn 4 it's: sn 4 -> sn 2 -> sn 1

All these paths intersect at snapshot sn 2, so we can now assign sn 2 as the new root of the tree:

          ┌────┐
          │sn 2├──────┐
          └──┬─┘      │
             │        │
          ┌──▼─┐   ┌──▼─┐
      ┌───│sn 3│   │sn 4│
      │   └────┘   └────┘
   ┌──▼─┐
   │sn 5│
   └────┘

*/
func (t *Tree) reduceTree() (res bool) {
	if len(t.possibleRoots) == 0 {
		return
	}
	firstHead := t.attached[t.headIds[0]]
	if firstHead.IsSnapshot && len(t.headIds) == 1 {
		t.clearPossibleRoots()
		t.makeRootAndRemove(firstHead)
		return true
	}
	curSnapshot, ok := t.attached[firstHead.SnapshotId]
	if !ok {
		log.Error("snapshot not found in tree", zap.String("snapshotId", t.attached[t.headIds[0]].SnapshotId))
		return false
	}
	if len(t.headIds) == 1 {
		t.clearPossibleRoots()
		t.makeRootAndRemove(curSnapshot)
		return true
	}
	// Gather snapshots from first head to root
	var path []*Change
	for curSnapshot.Id != t.root.Id {
		curSnapshot.visited = true
		path = append(path, curSnapshot)
		curSnapshot, ok = t.attached[curSnapshot.SnapshotId]
		if !ok {
			log.Error("snapshot not found in tree", zap.String("snapshotId", curSnapshot.SnapshotId))
			return false
		}
	}
	path = append(path, t.root)
	t.root.visited = true

	// Find the intersection point for snapshot paths of all heads. It will be the latest common snapshot for all those heads
	maxIdx := 0
	for i := 1; i < len(t.headIds); i++ {
		headSnapshot := t.attached[t.headIds[i]].SnapshotId
		cur, ok := t.attached[headSnapshot]
		if !ok {
			log.Error("snapshot not found in tree", zap.String("snapshotId", t.attached[t.headIds[i]].SnapshotId))
			return false
		}
		for {
			if cur.visited {
				// TODO: we may use counters here but it is not necessary
				idx := slices.IndexFunc(path, func(c *Change) bool {
					return c.Id == cur.Id
				})
				if idx > maxIdx {
					maxIdx = idx
				}
				break
			}
			cur, ok = t.attached[cur.SnapshotId]
			if !ok {
				log.Error("snapshot not found in tree", zap.String("snapshotId", cur.SnapshotId))
				return false
			}
		}
	}
	for _, c := range path {
		c.visited = false
	}
	t.clearPossibleRoots()
	t.makeRootAndRemove(path[maxIdx])
	return true
}
