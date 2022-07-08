package exampledocument

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/acltree"
)

// example ->

type documentStateBuilder struct {
	tree          *acltree.Tree
	aclState      *acltree.aclState // TODO: decide if this is needed or not
	stateProvider InitialStateProvider
}

func newDocumentStateBuilder(stateProvider InitialStateProvider) *documentStateBuilder {
	return &documentStateBuilder{
		stateProvider: stateProvider,
	}
}

func (d *documentStateBuilder) init(aclState *acltree.aclState, tree *acltree.Tree) {
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

	t.Iterate(startId, func(c *acltree.Change) (isContinue bool) {
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
	// TODO: we should have the same logic as in ACLStateBuilder, that means we should either pass in both methods state from the outside or save the state inside the builder
	d.tree.Iterate(fromId, func(c *acltree.Change) (isContinue bool) {
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
