package objecttree

type flusher interface {
	markNewChange(ch *Change)
	flushAfterBuild(t *objectTree) error
	flush(t *objectTree) error
}

type defaultFlusher struct {
}

func (d *defaultFlusher) markNewChange(ch *Change) {
}

func (d *defaultFlusher) flushAfterBuild(t *objectTree) error {
	t.tree.reduceTree()
	return nil
}

func (d *defaultFlusher) flush(t *objectTree) error {
	return nil
}

type newChangeFlusher struct {
	newChanges []*Change
}

func (n *newChangeFlusher) markNewChange(ch *Change) {
	ch.IsNew = true
	n.newChanges = append(n.newChanges, ch)
}

func (n *newChangeFlusher) flushAfterBuild(t *objectTree) error {
	return nil
}

func (n *newChangeFlusher) flush(t *objectTree) error {
	for _, ch := range n.newChanges {
		ch.IsNew = false
	}
	t.tree.reduceTree()
	return nil
}
