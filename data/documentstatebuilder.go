package data

import (
	"fmt"
)

type documentStateBuilder struct {
	tree          *Tree
	aclState      *ACLState // TODO: decide if this is needed or not
	stateProvider InitialStateProvider
}

func newDocumentStateBuilder(tree *Tree, state *ACLState, stateProvider InitialStateProvider) *documentStateBuilder {
	return &documentStateBuilder{
		tree:          tree,
		aclState:      state,
		stateProvider: stateProvider,
	}
}

// TODO: we should probably merge the two builders into one
func (d *documentStateBuilder) build() (s DocumentState, err error) {
	var (
		startId string
		count   int
	)
	rootChange := d.tree.Root()

	if rootChange.DecryptedDocumentChange == nil {
		err = fmt.Errorf("root doesn't have decrypted change")
		return
	}

	s, err = d.stateProvider.ProvideFromInitialChange(rootChange.DecryptedDocumentChange, rootChange.Id)
	if err != nil {
		return
	}

	t := d.tree
	startId = rootChange.Id

	t.Iterate(startId, func(c *Change) (isContinue bool) {
		count++
		if startId == c.Id {
			return true
		}
		if c.DecryptedDocumentChange != nil {
			_, err = s.ApplyChange(c.DecryptedDocumentChange, c.Id)
			if err != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		return
	}
	return s, err
}
