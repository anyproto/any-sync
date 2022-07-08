package acltree

type ACLContext struct {
	Tree     *Tree
	ACLState *aclState
	DocState DocumentState
}

func createTreeFromThread(t threadmodels.Thread, fromStart bool) (*Tree, error) {
	treeBuilder := NewTreeBuilder(t, threadmodels.NewEd25519Decoder())
	treeBuilder.Init()
	return treeBuilder.Build(fromStart)
}

func createACLStateFromThread(
	t threadmodels.Thread,
	identity string,
	key threadmodels.EncryptionPrivKey,
	decoder threadmodels.SigningPubKeyDecoder,
	provider InitialStateProvider,
	fromStart bool) (*ACLContext, error) {
	tree, err := createTreeFromThread(t, fromStart)
	if err != nil {
		return nil, err
	}

	accountData := &AccountData{
		Identity: identity,
		EncKey:   key,
	}

	aclTreeBuilder := NewACLTreeBuilder(t, decoder)
	aclTreeBuilder.Init()
	aclTree, err := aclTreeBuilder.Build()
	if err != nil {
		return nil, err
	}

	if !fromStart {
		snapshotValidator := NewSnapshotValidator(decoder, accountData)
		snapshotValidator.Init(aclTree)
		valid, err := snapshotValidator.ValidateSnapshot(tree.root)
		if err != nil {
			return nil, err
		}
		if !valid {
			// TODO: think about what to do if the snapshot is invalid - should we rebuild the tree without it
			return createACLStateFromThread(t, identity, key, decoder, provider, true)
		}
	}

	aclBuilder := NewACLStateBuilder(decoder, accountData)
	err = aclBuilder.Init(tree)
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
	provider InitialStateProvider,
	decoder threadmodels.SigningPubKeyDecoder) (*ACLContext, error) {
	context, err := createACLStateFromThread(t, identity, key, decoder, provider, false)
	if err != nil {
		return nil, err
	}

	docStateBuilder := newDocumentStateBuilder(provider)
	docStateBuilder.init(context.ACLState, context.Tree)
	docState, err := docStateBuilder.build()
	if err != nil {
		return nil, err
	}
	context.DocState = docState

	return context, nil
}
