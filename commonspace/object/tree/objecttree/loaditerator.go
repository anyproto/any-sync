package objecttree

import (
	"context"
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/slice"
)

type rawCacheEntry struct {
	change  *Change
	removed bool
	nextSet bool
	size    int
}

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
	storage      Storage
	builder      ChangeBuilder
	cache        map[string]rawCacheEntry
	idStack      []string
	heads        []string
	lastHeads    []string
	snapshotPath []string
	orderId      string
	root         *treechangeproto.RawTreeChangeWithId
	isExhausted  bool
}

func newLoadIterator(root *treechangeproto.RawTreeChangeWithId, snapshotPath []string, storage Storage, builder ChangeBuilder) *loadIterator {
	return &loadIterator{
		storage:      storage,
		builder:      builder,
		cache:        make(map[string]rawCacheEntry),
		snapshotPath: snapshotPath,
		root:         root,
	}
}

func (l *loadIterator) NextBatch(maxSize int) (batch IteratorBatch, err error) {
	batch.Root = l.root
	batch.SnapshotPath = l.snapshotPath
	var curSize int
	if l.isExhausted {
		return
	}
	l.isExhausted = true
	err = l.storage.GetAfterOrder(context.Background(), l.orderId, func(ctx context.Context, c StorageChange) (shouldContinue bool, err error) {
		l.orderId = c.OrderId
		rawEntry, ok := l.cache[c.Id]
		// if there are no such entry in cache continue
		if !ok {
			return true, nil
		}
		if rawEntry.removed {
			batch.Heads = slice.DiscardFromSlice(batch.Heads, func(s string) bool {
				return slices.Contains(c.PrevIds, s)
			})
			if !slices.Contains(batch.Heads, c.Id) {
				batch.Heads = append(batch.Heads, c.Id)
			}
			return true, nil
		}
		if curSize+rawEntry.size >= maxSize && len(batch.Batch) != 0 {
			l.isExhausted = false
			return false, nil
		}
		curSize += rawEntry.size

		cp := make([]byte, 0, len(c.RawChange))
		cp = append(cp, c.RawChange...)
		batch.Batch = append(batch.Batch, &treechangeproto.RawTreeChangeWithId{
			RawChange: cp,
			Id:        c.Id,
		})
		batch.Heads = slice.DiscardFromSlice(batch.Heads, func(s string) bool {
			return slices.Contains(c.PrevIds, s)
		})
		if !slices.Contains(batch.Heads, c.Id) {
			batch.Heads = append(batch.Heads, c.Id)
		}
		return true, nil
	})
	if err != nil {
		return
	}
	l.lastHeads = batch.Heads
	return
}

func (l *loadIterator) load(commonSnapshot string, heads, breakpoints []string) (err error) {
	ctx := context.Background()
	cs, err := l.storage.Get(ctx, commonSnapshot)
	if err != nil {
		return
	}
	rawCh := &treechangeproto.RawTreeChangeWithId{}
	err = l.storage.GetAfterOrder(ctx, cs.OrderId, func(ctx context.Context, change StorageChange) (shouldContinue bool, err error) {
		rawCh.Id = change.Id
		rawCh.RawChange = change.RawChange
		ch, err := l.builder.UnmarshallReduced(rawCh)
		if err != nil {
			return false, err
		}
		l.cache[change.Id] = rawCacheEntry{
			change: ch,
			size:   len(change.RawChange),
		}
		return true, nil
	})
	if err != nil {
		return
	}
	existingBreakpoints := make([]string, 0, len(breakpoints))
	for _, b := range breakpoints {
		_, ok := l.cache[b]
		if !ok {
			continue
		}
		existingBreakpoints = append(existingBreakpoints, b)
	}
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
				// this should not happen
				return fmt.Errorf("entry %s not found in cache", id)
			}
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
	l.orderId = cs.OrderId
	l.lastHeads = []string{cs.Id}
	return nil
}
