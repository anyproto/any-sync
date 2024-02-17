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
	var (
		beforeId = params.BeforeId
		include  = params.IncludeBeforeId
		full     = params.BuildFullTree
	)
	h.treeBuilder.Reset()
	if full {
		h.tree, err = h.treeBuilder.BuildFull()
		return
	}
	if beforeId == h.Id() && !include {
		return ErrLoadBeforeRoot
	}

	if params.HeadIds != nil {
		h.tree, err = h.treeBuilder.build(params.HeadIds, nil, nil)
		return
	}
	heads := []string{beforeId}
	if beforeId == "" {
		heads, err = h.treeStorage.Heads()
		if err != nil {
			return
		}
	} else if !include {
		beforeChange, err := h.treeBuilder.loadChange(beforeId)
		if err != nil {
			return err
		}
		heads = beforeChange.PreviousIds
	}

	h.tree, err = h.treeBuilder.build(heads, nil, nil)
	return
}
