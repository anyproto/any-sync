package tree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"time"
)

type rawChangeLoader struct {
	treeStorage   storage.TreeStorage
	changeBuilder ChangeBuilder

	// buffers
	idStack []string
	cache   map[string]rawCacheEntry
}

type rawCacheEntry struct {
	change    *Change
	rawChange *treechangeproto.RawTreeChangeWithId
	position  int
}

func newRawChangeLoader(treeStorage storage.TreeStorage, changeBuilder ChangeBuilder) *rawChangeLoader {
	return &rawChangeLoader{
		treeStorage:   treeStorage,
		changeBuilder: changeBuilder,
	}
}

func (r *rawChangeLoader) LoadFromTree(t *Tree, breakpoints []string) ([]*treechangeproto.RawTreeChangeWithId, error) {
	var stack []*Change
	for _, h := range t.headIds {
		stack = append(stack, t.attached[h])
	}

	convert := func(chs []*Change) (rawChanges []*treechangeproto.RawTreeChangeWithId, err error) {
		for _, ch := range chs {
			var raw *treechangeproto.RawTreeChangeWithId
			raw, err = r.changeBuilder.BuildRaw(ch)
			if err != nil {
				return
			}
			rawChanges = append(rawChanges, raw)
		}
		return
	}

	// getting all changes that we visit
	var results []*Change
	rootVisited := false
	t.dfsPrev(
		stack,
		breakpoints,
		func(ch *Change) bool {
			results = append(results, ch)
			return true
		},
		func(visited []*Change) {
			if t.root.visited {
				rootVisited = true
			}
		},
	)

	// if we stopped at breakpoints or there are no breakpoints
	if !rootVisited || len(breakpoints) == 0 {
		// in this case we will add root if there are no breakpoints
		return convert(results)
	}

	// now starting from breakpoints
	stack = stack[:0]
	for _, h := range breakpoints {
		if c, exists := t.attached[h]; exists {
			stack = append(stack, c)
		}
	}

	// doing another dfs to get all changes before breakpoints, we need to exclude them from results
	// if we don't have some breakpoints we will just ignore them
	t.dfsPrev(
		stack,
		[]string{},
		func(ch *Change) bool {
			return true
		},
		func(visited []*Change) {
			results = discardFromSlice(results, func(change *Change) bool {
				return change.visited
			})
		},
	)

	// otherwise we want to exclude everything that wasn't in breakpoints
	return convert(results)
}

func (r *rawChangeLoader) LoadFromStorage(commonSnapshot string, heads, breakpoints []string) ([]*treechangeproto.RawTreeChangeWithId, error) {
	// resetting cache
	r.cache = make(map[string]rawCacheEntry)
	defer func() {
		r.cache = nil
	}()

	existingBreakpoints := make([]string, 0, len(breakpoints))
	for _, b := range breakpoints {
		entry, err := r.loadEntry(b)
		if err != nil {
			continue
		}
		entry.position = -1
		r.cache[b] = entry
		existingBreakpoints = append(existingBreakpoints, b)
	}
	r.cache[commonSnapshot] = rawCacheEntry{position: -1}

	dfs := func(
		commonSnapshot string,
		heads []string,
		startCounter int,
		shouldVisit func(counter int, mapExists bool) bool,
		visit func(entry rawCacheEntry) rawCacheEntry) bool {

		// resetting stack
		r.idStack = r.idStack[:0]
		r.idStack = append(r.idStack, heads...)

		commonSnapshotVisited := false
		var err error
		for len(r.idStack) > 0 {
			id := r.idStack[len(r.idStack)-1]
			r.idStack = r.idStack[:len(r.idStack)-1]

			entry, exists := r.cache[id]
			if !shouldVisit(entry.position, exists) {
				continue
			}
			if !exists {
				entry, err = r.loadEntry(id)
				if err != nil {
					continue
				}
			}

			// setting the counter when we visit
			r.cache[id] = visit(entry)

			for _, prev := range entry.change.PreviousIds {
				if prev == commonSnapshot {
					commonSnapshotVisited = true
					break
				}
				entry, exists = r.cache[prev]
				if !shouldVisit(entry.position, exists) {
					continue
				}
				r.idStack = append(r.idStack, prev)
			}
		}
		return commonSnapshotVisited
	}

	// preparing first pass
	r.idStack = append(r.idStack, heads...)
	var buffer []*treechangeproto.RawTreeChangeWithId

	rootVisited := dfs(commonSnapshot, heads, 0,
		func(counter int, mapExists bool) bool {
			return !mapExists
		},
		func(entry rawCacheEntry) rawCacheEntry {
			buffer = append(buffer, entry.rawChange)
			entry.position = len(buffer) - 1
			return entry
		})

	// checking if we stopped at breakpoints
	if !rootVisited {
		return buffer, nil
	}

	// if there are no breakpoints then we should load root also
	if len(breakpoints) == 0 {
		common, err := r.loadEntry(commonSnapshot)
		if err != nil {
			return nil, err
		}
		buffer = append(buffer, common.rawChange)
		return buffer, nil
	}

	// marking all visited as nil
	dfs(commonSnapshot, existingBreakpoints, len(buffer),
		func(counter int, mapExists bool) bool {
			return !mapExists || counter < len(buffer)
		},
		func(entry rawCacheEntry) rawCacheEntry {
			if entry.position != -1 {
				buffer[entry.position] = nil
			}
			entry.position = len(buffer) + 1
			return entry
		})

	// discarding visited
	buffer = discardFromSlice(buffer, func(change *treechangeproto.RawTreeChangeWithId) bool {
		return change == nil
	})

	return buffer, nil
}

func (r *rawChangeLoader) loadEntry(id string) (entry rawCacheEntry, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	rawChange, err := r.treeStorage.GetRawChange(ctx, id)
	if err != nil {
		return
	}

	change, err := r.changeBuilder.ConvertFromRaw(rawChange, false)
	if err != nil {
		return
	}
	entry = rawCacheEntry{
		change:    change,
		rawChange: rawChange,
	}
	return
}
