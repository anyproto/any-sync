package objecttree

import (
	"context"
	"errors"
)

var ErrLoadBeforeRoot = errors.New("can't load before root")

type HistoryTree interface {
	ReadableObjectTree
}

type historyTree struct {
	*objectTree
}

func (h *historyTree) rebuildFromStorage(params HistoryTreeParams) (err error) {
	err = h.rebuild(params)
	if err != nil {
		return
	}
	h.aclList.RLock()
	defer h.aclList.RUnlock()
	state := h.aclList.AclState()

	return h.readKeysFromAclState(state)
}

func (h *historyTree) rebuild(params HistoryTreeParams) (err error) {
	switch len(params.Heads) {
	case 0:
		h.tree, err = h.treeBuilder.BuildFull()
		return err
	case 1:
		if params.Heads[0] == "" {
			return h.rebuildCurrent()
		}
		if !params.IncludeBeforeId {
			return h.rebuildFromPrevious(params.Heads[0])
		}
		fallthrough
	default:
		return h.rebuildFromHeads(params.Heads)
	}
}

func (h *historyTree) rebuildFromHeads(heads []string) (err error) {
	h.tree, err = h.treeBuilder.build(treeBuilderOpts{
		useHeadsSnapshot: true,
		ourHeads:         heads,
	})
	return
}

func (h *historyTree) rebuildCurrent() (err error) {
	h.tree, err = h.treeBuilder.build(treeBuilderOpts{})
	return
}

func (h *historyTree) rebuildFromPrevious(beforeId string) (err error) {
	change, err := h.storage.Get(context.Background(), beforeId)
	if err != nil {
		return err
	}
	h.tree, err = h.treeBuilder.build(treeBuilderOpts{
		useHeadsSnapshot: true,
		ourHeads:         change.PrevIds,
	})
	return err
}
