package data

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/core/block/editor/state"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"
)

type ACLContext struct {
	Tree     *Tree
	ACLState *ACLState
	DocState *state.State
}

func createTreeFromThread(t threadmodels.Thread, fromStart bool) (*Tree, error) {
	treeBuilder := NewTreeBuilder(t, threadmodels.NewEd25519Decoder())
	return treeBuilder.Build(fromStart)
}

func createACLStateFromThread(
	t threadmodels.Thread,
	identity string,
	key threadmodels.EncryptionPrivKey,
	decoder threadmodels.SigningPubKeyDecoder,
	fromStart bool) (*ACLContext, error) {
	tree, err := createTreeFromThread(t, fromStart)
	if err != nil {
		return nil, err
	}

	aclTreeBuilder := NewACLTreeBuilder(t, decoder)
	aclTree, err := aclTreeBuilder.Build()
	if err != nil {
		return nil, err
	}

	if !fromStart {
		snapshotValidator := NewSnapshotValidator(aclTree, identity, key, decoder)
		valid, err := snapshotValidator.ValidateSnapshot(tree.root)
		if err != nil {
			return nil, err
		}
		if !valid {
			// TODO: think about what to do if the snapshot is invalid - should we rebuild the tree without it
			return createACLStateFromThread(t, identity, key, decoder, true)
		}
	}

	aclBuilder, err := NewACLStateBuilder(tree, identity, key, decoder)
	if err != nil {
		return nil, err
	}

	aclState, err := aclBuilder.Build()
	if err != nil {
		return nil, err
	}
	return &ACLContext{
		Tree:     tree,
		ACLState: aclState,
	}, nil
}

func createDocumentStateFromThread(
	t threadmodels.Thread,
	identity string,
	key threadmodels.EncryptionPrivKey,
	decoder threadmodels.SigningPubKeyDecoder) (*ACLContext, error) {
	context, err := createACLStateFromThread(t, identity, key, decoder, false)
	if err != nil {
		return nil, err
	}

	docStateBuilder := newDocumentStateBuilder(context.Tree, context.ACLState)
	docState, err := docStateBuilder.build()
	if err != nil {
		return nil, err
	}
	context.DocState = docState

	return context, nil
}
