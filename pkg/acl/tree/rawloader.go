package tree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
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
	rawChange *aclpb.RawChange
}

func newRawChangeLoader(treeStorage storage.TreeStorage, changeBuilder ChangeBuilder) *rawChangeLoader {
	return &rawChangeLoader{
		treeStorage:   treeStorage,
		changeBuilder: changeBuilder,
	}
}

func (r *rawChangeLoader) LoadFromTree(t *Tree, breakpoints []string) ([]*aclpb.RawChange, error) {
	var stack []*Change
	for _, h := range t.headIds {
		stack = append(stack, t.attached[h])
	}

	convert := func(chs []*Change) (rawChanges []*aclpb.RawChange, err error) {
		for _, ch := range chs {
			var marshalled []byte
			marshalled, err = ch.Content.Marshal()
			if err != nil {
				return
			}

			raw := &aclpb.RawChange{
				Payload:   marshalled,
				Signature: ch.Signature(),
				Id:        ch.Id,
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
		return convert(results)
	}

	// now starting from breakpoints
	stack = stack[:0]
	for _, h := range breakpoints {
		stack = append(stack, t.attached[h])
	}

	// doing another dfs to get all changes before breakpoints, we need to exclude them from results
	t.dfsPrev(
		stack,
		[]string{},
		func(ch *Change) bool {
			return true
		},
		func(visited []*Change) {
			discardFromSlice(results, func(change *Change) bool {
				return change.visited
			})
		},
	)

	// otherwise we want to exclude everything that wasn't in breakpoints
	return convert(results)
}

func (r *rawChangeLoader) loadEntry(id string) (entry rawCacheEntry, err error) {
	var ok bool
	if entry, ok = r.cache[id]; ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	rawChange, err := r.treeStorage.GetRawChange(ctx, id)
	if err != nil {
		return
	}

	change, err := r.changeBuilder.ConvertFromRaw(rawChange)
	if err != nil {
		return
	}
	entry = rawCacheEntry{
		change:    change,
		rawChange: rawChange,
	}
	r.cache[id] = entry
	return
}

func (r *rawChangeLoader) LoadFromStorage(commonSnapshot string, heads, breakpoints []string) ([]*aclpb.RawChange, error) {
	// initializing buffers
	r.idStack = r.idStack[:0]

	// updating map
	bufPosMap := make(map[string]int)
	for _, breakpoint := range breakpoints {
		bufPosMap[breakpoint] = -1
	}
	bufPosMap[commonSnapshot] = -1

	dfs := func(
		commonSnapshot string,
		heads []string,
		startCounter int,
		shouldVisit func(counter int, mapExists bool) bool,
		visit func(prevCounter int, entry rawCacheEntry) int) bool {

		commonSnapshotVisited := false
		for len(r.idStack) > 0 {
			id := r.idStack[len(r.idStack)-1]
			r.idStack = r.idStack[:len(r.idStack)-1]

			cnt, exists := bufPosMap[id]
			if !shouldVisit(cnt, exists) {
				continue
			}

			entry, err := r.loadEntry(id)
			if err != nil {
				continue
			}

			// setting the counter when we visit
			bufPosMap[id] = visit(cnt, entry)

			for _, prev := range entry.change.PreviousIds {
				if prev == commonSnapshot {
					commonSnapshotVisited = true
					break
				}
				cnt, exists = bufPosMap[prev]
				if !shouldVisit(cnt, exists) {
					continue
				}
				r.idStack = append(r.idStack, prev)
			}
		}
		return commonSnapshotVisited
	}

	// preparing first pass
	r.idStack = append(r.idStack, heads...)
	var buffer []*aclpb.RawChange

	rootVisited := dfs(commonSnapshot, heads, 0,
		func(counter int, mapExists bool) bool {
			return !mapExists
		},
		func(_ int, entry rawCacheEntry) int {
			buffer = append(buffer, entry.rawChange)
			return len(buffer) - 1
		})

	// checking if we stopped at breakpoints
	if !rootVisited || len(breakpoints) == 0 {
		return buffer, nil
	}

	// resetting stack
	r.idStack = r.idStack[:0]
	r.idStack = append(r.idStack, breakpoints...)

	// marking all visited as nil
	dfs(commonSnapshot, heads, len(buffer),
		func(counter int, mapExists bool) bool {
			return !mapExists || counter < len(buffer)
		},
		func(discardedPosition int, entry rawCacheEntry) int {
			if discardedPosition != -1 {
				buffer[discardedPosition] = nil
			}
			return len(buffer) + 1
		})
	discardFromSlice(buffer, func(change *aclpb.RawChange) bool {
		return change == nil
	})

	return buffer, nil
}

func discardFromSlice[T any](elements []T, isDiscarded func(T) bool) {
	var (
		finishedIdx = 0
		currentIdx  = 0
	)
	for currentIdx < len(elements) {
		if !isDiscarded(elements[currentIdx]) && finishedIdx != currentIdx {
			elements[finishedIdx] = elements[currentIdx]
			finishedIdx++
		}
		currentIdx++
	}
	elements = elements[:finishedIdx]
}
