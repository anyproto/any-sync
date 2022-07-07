package data

import (
	"fmt"
)

// example ->

type documentStateBuilder struct {
	tree          *Tree
	aclState      *ACLState // TODO: decide if this is needed or not
	stateProvider InitialStateProvider
}

func newDocumentStateBuilder(stateProvider InitialStateProvider) *documentStateBuilder {
	return &documentStateBuilder{
		stateProvider: stateProvider,
	}
}

func (d *documentStateBuilder) init(aclState *ACLState, tree *Tree) {
	d.tree = tree
	d.aclState = aclState
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
			s, err = s.ApplyChange(c.DecryptedDocumentChange, c.Id)
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

func (d *documentStateBuilder) appendFrom(fromId string, init DocumentState) (s DocumentState, err error) {
	// TODO: we should do something like state copy probably
	s = init
	d.tree.Iterate(fromId, func(c *Change) (isContinue bool) {
		if c.Id == fromId {
			return true
		}
		if c.DecryptedDocumentChange != nil {
			s, err = s.ApplyChange(c.DecryptedDocumentChange, c.Id)
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
