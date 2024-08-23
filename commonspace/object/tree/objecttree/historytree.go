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
		return err
	}

	if len(params.Heads) == 1 {
		return h.rebuildWithSingleHead(params.IncludeBeforeId, params.Heads[0])
	}

	h.tree, err = h.treeBuilder.build(params.Heads, nil, nil)
	return err
}

func (h *historyTree) rebuildWithSingleHead(includeBeforeId bool, head string) (err error) {
	if head == "" {
		return h.rebuildWithEmptyHead()
	}
	if !includeBeforeId {
		return h.rebuildWithPreviousHead(head)
	}
	h.tree, err = h.treeBuilder.build([]string{head}, nil, nil)
	return err
}

func (h *historyTree) rebuildWithEmptyHead() (err error) {
	heads, err := h.treeStorage.Heads()
	if err != nil {
		return err
	}
	h.tree, err = h.treeBuilder.build(heads, nil, nil)
	return err
}

func (h *historyTree) rebuildWithPreviousHead(head string) (err error) {
	change, err := h.treeBuilder.loadChange(head)
	if err != nil {
		return err
	}
	h.tree, err = h.treeBuilder.build(change.PreviousIds, nil, nil)
	return err
}
