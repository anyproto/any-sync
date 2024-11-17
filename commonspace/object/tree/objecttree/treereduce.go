package objecttree

import (
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
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
	cur, ok := t.attached[t.attached[t.headIds[0]].SnapshotId]
	if !ok {
		log.Error("snapshot not found in tree", zap.String("snapshotId", t.attached[t.headIds[0]].SnapshotId))
		return false
	}
	if len(t.headIds) == 1 {
		t.clearPossibleRoots()
		t.makeRootAndRemove(cur)
		return true
	}
	// gathering snapshots from first head to root
	var path []*Change
	for cur.Id != t.root.Id {
		path = append(path, cur)
		cur, ok = t.attached[cur.SnapshotId]
		if !ok {
			log.Error("snapshot not found in tree", zap.String("snapshotId", cur.SnapshotId))
			return false
		}
		cur.visited = true
	}
	path = append(path, t.root)
	t.root.visited = true
	// checking where paths from other heads intersect path
	maxIdx := 0
	for i := 1; i < len(t.headIds); i++ {
		cur, ok := t.attached[t.attached[t.headIds[i]].SnapshotId]
		if !ok {
			log.Error("snapshot not found in tree", zap.String("snapshotId", t.attached[t.headIds[i]].SnapshotId))
			return false
		}
		for cur.Id != t.root.Id {
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
