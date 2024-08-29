package objecttree

type Flusher interface {
	MarkNewChange(ch *Change)
	FlushAfterBuild(t *objectTree) error
	Flush(t *objectTree) error
}

type defaultFlusher struct {
}

func (d *defaultFlusher) MarkNewChange(ch *Change) {
}

func (d *defaultFlusher) FlushAfterBuild(t *objectTree) error {
	t.tree.reduceTree()
	return nil
}

func (d *defaultFlusher) Flush(t *objectTree) error {
	return nil
}

func MarkNewChangeFlusher() Flusher {
	return &newChangeFlusher{}
}

type newChangeFlusher struct {
	newChanges []*Change
}

func (n *newChangeFlusher) MarkNewChange(ch *Change) {
	ch.IsNew = true
	n.newChanges = append(n.newChanges, ch)
}

func (n *newChangeFlusher) FlushAfterBuild(t *objectTree) error {
	return nil
}

func (n *newChangeFlusher) Flush(t *objectTree) error {
	for _, ch := range n.newChanges {
		ch.IsNew = false
	}
	t.tree.reduceTree()
	return nil
}
