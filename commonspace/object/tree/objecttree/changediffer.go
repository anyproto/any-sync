package objecttree

import (
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/util/slice"
)

type (
	treeBuilderFunc func(heads []string) (ReadableObjectTree, error)
	onRemoveFunc    func(ids []string)
)

type diffChange struct {
	Id          string
	PreviousIds []string
	Previous    []*diffChange
	visited     bool
}

func newDiffChange(id string, previousIds []string) *diffChange {
	return &diffChange{
		Id:          id,
		PreviousIds: previousIds,
	}
}

type ChangeDiffer struct {
	known      map[string]struct{}
	attached   map[string]*diffChange
	waitList   map[string][]*diffChange
	visitedBuf []*diffChange
}

func NewChangeDiffer(tree ReadableObjectTree) (*ChangeDiffer, error) {
	diff := &ChangeDiffer{
		known:    make(map[string]struct{}),
		attached: make(map[string]*diffChange),
		waitList: make(map[string][]*diffChange),
	}
	if tree == nil {
		return diff, nil
	}
	err := tree.IterateRoot(nil, func(c *Change) (isContinue bool) {
		diff.known[c.Id] = struct{}{}
		diff.add(newDiffChange(c.Id, c.PreviousIds))
		return true
	})
	if err != nil {
		return nil, err
	}
	return diff, nil
}

func (d *ChangeDiffer) RemoveBefore(ids []string, seenHeads []string) (removed []string, notFound []string, heads []string) {
	var (
		attached    []*diffChange
		attachedIds []string
	)
	for _, id := range ids {
		if ch, ok := d.attached[id]; ok {
			attached = append(attached, ch)
			attachedIds = append(attachedIds, id)
			continue
		}
		if _, ok := d.known[id]; !ok {
			notFound = append(notFound, id)
		}
	}
	heads = make([]string, 0, len(seenHeads))
	heads = append(heads, seenHeads...)
	d.dfsPrev(attached, func(ch *diffChange) (isContinue bool) {
		heads = append(heads, ch.Id)
		heads = append(heads, ch.PreviousIds...)
		removed = append(removed, ch.Id)
		return true
	}, func() {
		for _, ch := range removed {
			delete(d.attached, ch)
		}
		for _, ch := range d.attached {
			ch.Previous = slice.DiscardFromSlice(ch.Previous, func(change *diffChange) bool {
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

func (d *ChangeDiffer) dfsPrev(stack []*diffChange, visit func(ch *diffChange) (isContinue bool), afterVisit func()) {
	d.visitedBuf = d.visitedBuf[:0]
	defer func() {
		if afterVisit != nil {
			afterVisit()
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
		d.add(newDiffChange(ch.Id, ch.PreviousIds))
	}
	return
}

func (d *ChangeDiffer) add(change *diffChange) {
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
	differ    *ChangeDiffer
	notFound  map[string]struct{}
	onRemove  func(ids []string)
	heads     []string
	seenHeads []string
}

func NewDiffManager(initHeads, curHeads []string, treeBuilder treeBuilderFunc, onRemove onRemoveFunc) (*DiffManager, error) {
	allHeads := make([]string, 0, len(initHeads)+len(curHeads))
	allHeads = append(allHeads, initHeads...)
	allHeads = append(allHeads, curHeads...)
	readableTree, err := treeBuilder(allHeads)
	if err != nil {
		return nil, err
	}
	differ, err := NewChangeDiffer(readableTree)
	if err != nil {
		return nil, err
	}
	return &DiffManager{
		differ:    differ,
		heads:     curHeads,
		seenHeads: initHeads,
		onRemove:  onRemove,
		notFound:  make(map[string]struct{}),
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
