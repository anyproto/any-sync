package objecttree

import (
	"errors"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
)

var ErrLoadBeforeRoot = errors.New("can't load before root")

type HistoryTree interface {
	RWLocker

	Id() string
	Root() *Change
	Heads() []string
	IterateFrom(id string, convert ChangeConvertFunc, iterate ChangeIterateFunc) error
	GetChange(string) (*Change, error)
	Header() *treechangeproto.RawTreeChangeWithId
	UnmarshalledHeader() *Change
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
	return
}
