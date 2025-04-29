package objecttree

type objectTreeDebug struct {
}

type DebugInfo struct {
	TreeLen      int
	TreeString   string
	Graphviz     string
	Heads        []string
	SnapshotPath []string
}

func (o objectTreeDebug) debugInfo(ot *objectTree, parser DescriptionParser) (di DebugInfo, err error) {
	di = DebugInfo{}
	di.Graphviz, err = ot.tree.Graph(parser)
	if err != nil {
		return
	}
	di.TreeString = ot.tree.String()
	di.TreeLen = ot.tree.Len()
	di.Heads = ot.Heads()
	di.SnapshotPath, _ = ot.SnapshotPath()
	return
}
