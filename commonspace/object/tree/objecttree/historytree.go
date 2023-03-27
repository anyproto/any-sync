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

func (h *historyTree) rebuildFromStorage(beforeId string, include bool) (err error) {
	ot := h.objectTree
	ot.treeBuilder.Reset()
	if beforeId == ot.Id() && !include {
		return ErrLoadBeforeRoot
	}

	heads := []string{beforeId}
	if beforeId == "" {
		heads, err = ot.treeStorage.Heads()
		if err != nil {
			return
		}
	} else if !include {
		beforeChange, err := ot.treeBuilder.loadChange(beforeId)
		if err != nil {
			return err
		}
		heads = beforeChange.PreviousIds
	}

	ot.tree, err = ot.treeBuilder.build(heads, nil, nil)
	if err != nil {
		return
	}
	ot.aclList.RLock()
	defer ot.aclList.RUnlock()
	state := ot.aclList.AclState()

	return ot.readKeysFromAclState(state)
}
