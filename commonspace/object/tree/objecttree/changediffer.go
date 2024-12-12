package objecttree

type (
	hasChangesFunc  func(ids ...string) bool
	treeBuilderFunc func(heads []string) (ReadableObjectTree, error)
	onRemoveFunc    func(ids []string)
)

type ChangeDiffer struct {
	*Tree
	hasChanges hasChangesFunc
}

func NewChangeDiffer(tree *Tree, hasChanges hasChangesFunc) *ChangeDiffer {
	return &ChangeDiffer{
		Tree:       tree,
		hasChanges: hasChanges,
	}
}

func (d *ChangeDiffer) RemoveBefore(ids []string) (removed []string, notFound []string) {
	var attached []*Change
	for _, id := range ids {
		if ch, ok := d.attached[id]; ok {
			attached = append(attached, ch)
			continue
		}
		// check if we have it at the bottom
		if !d.hasChanges(id) {
			notFound = append(notFound, id)
		}
	}
	d.Tree.dfsPrev(attached, nil, func(ch *Change) (isContinue bool) {
		removed = append(removed, ch.Id)
		return true
	}, nil)
	for _, ch := range removed {
		delete(d.attached, ch)
	}
	return
}

func (d *ChangeDiffer) Add(changes ...*Change) (added []*Change) {
	_, added = d.Tree.Add(changes...)
	return
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
	differ := NewChangeDiffer(readableTree.Tree(), readableTree.HasChanges)
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
	removed, _ := d.differ.RemoveBefore(d.heads)
	d.onRemove(removed)
}

func (d *DiffManager) Remove(ids []string) {
	removed, notFound := d.differ.RemoveBefore(ids)
	for _, id := range notFound {
		d.notFound[id] = struct{}{}
	}
	d.onRemove(removed)
}

func (d *DiffManager) Update(objTree ObjectTree) {
	var (
		toAdd    = make([]*Change, 0, objTree.Len())
		toRemove []string
	)
	objTree.Tree().iterate(objTree.Root(), func(ch *Change) bool {
		toAdd = append(toAdd, &Change{
			Id:          ch.Id,
			PreviousIds: ch.PreviousIds,
		})
		if _, ok := d.notFound[ch.Id]; ok {
			toRemove = append(toRemove, ch.Id)
		}
		return true
	})
	d.differ.Add(toAdd...)
	d.heads = make([]string, 0, len(d.heads))
	d.heads = append(d.heads, objTree.Heads()...)
	removed, _ := d.differ.RemoveBefore(toRemove)
	d.onRemove(removed)
}
