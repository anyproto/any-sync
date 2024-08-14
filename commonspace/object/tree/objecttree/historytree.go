package objecttree

import (
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
	h.treeBuilder.Reset()
	if len(params.Heads) == 0 {
		h.tree, err = h.treeBuilder.BuildFull()
		return
	}

	if len(params.Heads) == 1 && params.Heads[0] == "" {
		params.Heads, err = h.treeStorage.Heads()
		if err != nil {
			return
		}
	}
	if !params.IncludeBeforeId && len(params.Heads) == 1 {
		beforeChange, err := h.treeBuilder.loadChange(params.Heads[0])
		if err != nil {
			return err
		}
		params.Heads = beforeChange.PreviousIds
	}
	h.tree, err = h.treeBuilder.build(params.Heads, nil, nil)
	return
}
