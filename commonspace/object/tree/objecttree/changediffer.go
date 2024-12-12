package objecttree

type hasChangesFunc func(ids ...string) bool

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

func (d *ChangeDiffer) RemoveBefore(ids []string) (removed []*Change, notFound []string) {
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
		removed = append(removed, ch)
		return true
	}, nil)
	for _, ch := range removed {
		delete(d.attached, ch.Id)
	}
	return
}

func (d *ChangeDiffer) Add(changes ...*Change) (added []*Change) {
	_, added = d.Tree.Add(changes...)
	return
}
