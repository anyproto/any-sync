package objecttree

import (
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/util/slice"
)

type (
	hasChangesFunc  func(ids ...string) bool
	treeBuilderFunc func(heads []string) (ReadableObjectTree, error)
	onRemoveFunc    func(ids []string)
)

type ChangeDiffer struct {
	hasChanges hasChangesFunc
	attached   map[string]*Change
	waitList   map[string][]*Change
	visitedBuf []*Change
}

func NewChangeDiffer(tree ReadableObjectTree, hasChanges hasChangesFunc) (*ChangeDiffer, error) {
	diff := &ChangeDiffer{
		hasChanges: hasChanges,
		attached:   make(map[string]*Change),
		waitList:   make(map[string][]*Change),
	}
	if tree == nil {
		return diff, nil
	}
	err := tree.IterateRoot(nil, func(c *Change) (isContinue bool) {
		diff.add(&Change{
			Id:          c.Id,
			PreviousIds: c.PreviousIds,
		})
		return true
	})
	if err != nil {
		return nil, err
	}
	return diff, nil
}

func (d *ChangeDiffer) RemoveBefore(ids []string, seenHeads []string) (removed []string, notFound []string, heads []string) {
	var (
		attached    []*Change
		attachedIds []string
	)
	for _, id := range ids {
		if ch, ok := d.attached[id]; ok {
			attached = append(attached, ch)
			attachedIds = append(attachedIds, id)
			continue
		}
		// check if we have it at the bottom
		if !d.hasChanges(id) {
			notFound = append(notFound, id)
		}
	}
	heads = make([]string, 0, len(seenHeads))
	heads = append(heads, seenHeads...)
	d.dfsPrev(attached, func(ch *Change) (isContinue bool) {
		heads = append(heads, ch.Id)
		heads = append(heads, ch.PreviousIds...)
		removed = append(removed, ch.Id)
		return true
	}, func(changes []*Change) {
		for _, ch := range removed {
			delete(d.attached, ch)
		}
		for _, ch := range d.attached {
			ch.Previous = slice.DiscardFromSlice(ch.Previous, func(change *Change) bool {
				return change.visited
			})
		}
	})
	slices.Sort(heads)
	heads = slice.RemoveRepeatedSorted(heads)
	heads = append(heads, attachedIds...)
	heads = append(heads, seenHeads...)
	slices.Sort(heads)
	heads = slice.RemoveUniqueElementsSorted(heads)
	heads = slice.DiscardDuplicatesSorted(heads)
	return
}

func (d *ChangeDiffer) dfsPrev(stack []*Change, visit func(ch *Change) (isContinue bool), afterVisit func([]*Change)) {
	d.visitedBuf = d.visitedBuf[:0]
	defer func() {
		if afterVisit != nil {
			afterVisit(d.visitedBuf)
		}
		for _, ch := range d.visitedBuf {
			ch.visited = false
		}
	}()
	for len(stack) > 0 {
		ch := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if ch.visited {
			continue
		}
		ch.visited = true
		d.visitedBuf = append(d.visitedBuf, ch)
		for _, prevCh := range ch.Previous {
			if !prevCh.visited {
				stack = append(stack, prevCh)
			}
		}
		if !visit(ch) {
			return
		}
	}
}

func (d *ChangeDiffer) Add(changes ...*Change) {
	for _, ch := range changes {
		d.add(ch)
	}
	return
}

func (d *ChangeDiffer) add(change *Change) {
	_, exists := d.attached[change.Id]
	if exists {
		return
	}
	d.attached[change.Id] = change
	wl, exists := d.waitList[change.Id]
	if exists {
		for _, ch := range wl {
			ch.Previous = append(ch.Previous, change)
		}
		delete(d.waitList, change.Id)
	}
	for _, id := range change.PreviousIds {
		prev, exists := d.attached[id]
		if exists {
			change.Previous = append(change.Previous, prev)
			continue
		}
		wl := d.waitList[id]
		wl = append(wl, change)
		d.waitList[id] = wl
	}
}

type DiffManager struct {
	differ       *ChangeDiffer
	readableTree ReadableObjectTree
	notFound     map[string]struct{}
	onRemove     func(ids []string)
	heads        []string
	seenHeads    []string
}

func NewDiffManager(initHeads, curHeads []string, treeBuilder treeBuilderFunc, onRemove onRemoveFunc) (*DiffManager, error) {
	allHeads := make([]string, 0, len(initHeads)+len(curHeads))
	allHeads = append(allHeads, initHeads...)
	allHeads = append(allHeads, curHeads...)
	readableTree, err := treeBuilder(allHeads)
	if err != nil {
		return nil, err
	}
	differ, err := NewChangeDiffer(readableTree, readableTree.HasChanges)
	if err != nil {
		return nil, err
	}
	return &DiffManager{
		differ:       differ,
		heads:        curHeads,
		seenHeads:    initHeads,
		onRemove:     onRemove,
		readableTree: readableTree,
		notFound:     make(map[string]struct{}),
	}, nil
}

func (d *DiffManager) Init() {
	removed, _, seenHeads := d.differ.RemoveBefore(d.seenHeads, []string{})
	d.seenHeads = seenHeads
	d.onRemove(removed)
}

func (d *DiffManager) SeenHeads() []string {
	return d.seenHeads
}

func (d *DiffManager) Remove(ids []string) {
	removed, notFound, seenHeads := d.differ.RemoveBefore(ids, d.seenHeads)
	for _, id := range notFound {
		d.notFound[id] = struct{}{}
	}
	d.seenHeads = seenHeads
	d.onRemove(removed)
}

func (d *DiffManager) Add(change *Change) {
	d.differ.Add(change)
}

func (d *DiffManager) GetIds() []string {
	ids := make([]string, 0, len(d.differ.attached))
	for id := range d.differ.attached {
		ids = append(ids, id)
	}
	return ids
}

func (d *DiffManager) Update(objTree ObjectTree) {
	var (
		toAdd    = make([]*Change, 0, objTree.Len())
		toRemove []string
	)
	err := objTree.IterateRoot(nil, func(ch *Change) bool {
		if ch.IsNew {
			toAdd = append(toAdd, &Change{
				Id:          ch.Id,
				PreviousIds: ch.PreviousIds,
			})
		}
		if _, ok := d.notFound[ch.Id]; ok {
			toRemove = append(toRemove, ch.Id)
			delete(d.notFound, ch.Id)
		}
		return true
	})
	if err != nil {
		log.Warn("error while iterating over object tree", zap.Error(err))
		return
	}
	d.differ.Add(toAdd...)
	d.heads = make([]string, 0, len(d.heads))
	d.heads = append(d.heads, objTree.Heads()...)
	removed, _, seenHeads := d.differ.RemoveBefore(toRemove, d.seenHeads)
	d.seenHeads = seenHeads
	d.onRemove(removed)
}
