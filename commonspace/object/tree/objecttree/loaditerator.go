package objecttree

import (
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/slice"
)

type LoadIterator interface {
	NextBatch(maxSize int) (batch IteratorBatch, err error)
}

type IteratorBatch struct {
	Batch        []*treechangeproto.RawTreeChangeWithId
	Heads        []string
	SnapshotPath []string
	Root         *treechangeproto.RawTreeChangeWithId
}

type loadIterator struct {
	loader       *rawChangeLoader
	cache        map[string]rawCacheEntry
	idStack      []string
	heads        []string
	lastHeads    []string
	snapshotPath []string
	root         *Change
	lastChange   *Change
	iter         *iterator
	isExhausted  bool
}

func newLoadIterator(loader *rawChangeLoader, snapshotPath []string) *loadIterator {
	return &loadIterator{
		loader:       loader,
		cache:        make(map[string]rawCacheEntry),
		snapshotPath: snapshotPath,
	}
}

func (l *loadIterator) NextBatch(maxSize int) (batch IteratorBatch, err error) {
	l.iter = newIterator()
	defer freeIterator(l.iter)
	curSize := 0
	batch.Root = l.loader.Root()
	batch.Heads = l.lastHeads
	batch.SnapshotPath = l.snapshotPath
	if l.isExhausted {
		return
	}
	l.isExhausted = true
	l.iter.iterateSkip(l.root, l.lastChange, true, func(c *Change) (isContinue bool) {
		l.lastChange = c
		rawEntry := l.cache[c.Id]
		if rawEntry.removed {
			return true
		}
		if curSize+rawEntry.size > maxSize {
			l.isExhausted = false
			return false
		}
		curSize += rawEntry.size
		batch.Batch = append(batch.Batch, rawEntry.rawChange)
		batch.Heads = slice.DiscardFromSlice(batch.Heads, func(s string) bool {
			return slices.Contains(c.PreviousIds, s)
		})
		batch.Heads = append(batch.Heads, c.Id)
		return true
	})
	l.lastHeads = batch.Heads
	return
}

func (l *loadIterator) load(commonSnapshot string, heads, breakpoints []string) (err error) {
	existingBreakpoints := make([]string, 0, len(breakpoints))
	for _, b := range breakpoints {
		_, err := l.loader.loadEntry(b)
		if err != nil {
			continue
		}
		existingBreakpoints = append(existingBreakpoints, b)
	}
	loadedCs, err := l.loader.loadEntry(commonSnapshot)
	if err != nil {
		return err
	}
	l.cache[commonSnapshot] = loadedCs
	l.heads = heads

	dfs := func(
		commonSnapshot string,
		heads []string,
		shouldVisit func(entry rawCacheEntry, mapExists bool) bool,
		visit func(entry rawCacheEntry) rawCacheEntry) (err error) {

		// resetting stack
		l.idStack = l.idStack[:0]
		l.idStack = append(l.idStack, heads...)

		for len(l.idStack) > 0 {
			id := l.idStack[len(l.idStack)-1]
			l.idStack = l.idStack[:len(l.idStack)-1]

			entry, exists := l.cache[id]
			if !shouldVisit(entry, exists) {
				continue
			}
			if !exists {
				entry, err = l.loader.loadEntry(id)
				if err != nil {
					return
				}
			}
			// setting the counter when we visit
			entry = visit(entry)
			l.cache[id] = entry

			for _, prev := range entry.change.PreviousIds {
				prevEntry, exists := l.cache[prev]
				if !shouldVisit(prevEntry, exists) {
					continue
				}
				l.idStack = append(l.idStack, prev)
			}
		}
		return nil
	}

	l.idStack = append(l.idStack, heads...)
	// load everything
	err = dfs(commonSnapshot, heads,
		func(_ rawCacheEntry, mapExists bool) bool {
			return !mapExists
		},
		func(entry rawCacheEntry) rawCacheEntry {
			entry.position = 0
			return entry
		})
	if err != nil {
		return
	}
	// marking some changes as removed, not to send to anybody
	err = dfs(commonSnapshot, existingBreakpoints,
		func(entry rawCacheEntry, mapExists bool) bool {
			// only going through already loaded changes
			return mapExists && !entry.removed
		},
		func(entry rawCacheEntry) rawCacheEntry {
			entry.removed = true
			return entry
		})
	if err != nil {
		return
	}
	// setting next for all changes in batch
	err = dfs(commonSnapshot, heads,
		func(entry rawCacheEntry, mapExists bool) bool {
			return mapExists && !entry.nextSet
		},
		func(entry rawCacheEntry) rawCacheEntry {
			for _, id := range entry.change.PreviousIds {
				prevEntry, exists := l.cache[id]
				if !exists {
					continue
				}
				prev := prevEntry.change
				if len(prev.Next) == 0 || prev.Next[len(prev.Next)-1].Id <= entry.change.Id {
					prev.Next = append(prev.Next, entry.change)
				} else {
					insertIdx := 0
					for idx, el := range prev.Next {
						if el.Id >= entry.change.Id {
							insertIdx = idx
							break
						}
					}
					prev.Next = append(prev.Next[:insertIdx+1], prev.Next[insertIdx:]...)
					prev.Next[insertIdx] = entry.change
				}
			}
			entry.nextSet = true
			return entry
		})
	if err != nil {
		return
	}
	l.lastHeads = []string{l.root.Id}
	l.lastChange = loadedCs.change
	l.root = loadedCs.change
	return nil
}
