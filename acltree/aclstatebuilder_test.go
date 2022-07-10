package acltree

import (
	"testing"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/testutils/threadbuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"

	"github.com/stretchr/testify/assert"
)

type ACLContext struct {
	Tree     *Tree
	ACLState *ACLState
}

func createTreeFromThread(t thread.Thread, fromStart bool) (*Tree, error) {
	treeBuilder := newTreeBuilder(t, keys.NewEd25519Decoder())
	treeBuilder.Init()
	return treeBuilder.Build(fromStart)
}

func createACLStateFromThread(
	t thread.Thread,
	identity string,
	key keys.EncryptionPrivKey,
	decoder keys.SigningPubKeyDecoder,
	fromStart bool) (*ACLContext, error) {
	tree, err := createTreeFromThread(t, fromStart)
	if err != nil {
		return nil, err
	}

	accountData := &account.AccountData{
		Identity: identity,
		EncKey:   key,
	}

	aclTreeBuilder := newACLTreeBuilder(t, decoder)
	aclTreeBuilder.Init()
	aclTree, err := aclTreeBuilder.Build()
	if err != nil {
		return nil, err
	}

	if !fromStart {
		snapshotValidator := newSnapshotValidator(decoder, accountData)
		snapshotValidator.Init(aclTree)
		valid, err := snapshotValidator.ValidateSnapshot(tree.root)
		if err != nil {
			return nil, err
		}
		if !valid {
			// TODO: think about what to do if the snapshot is invalid - should we rebuild the Tree without it
			return createACLStateFromThread(t, identity, key, decoder, true)
		}
	}

	aclBuilder := newACLStateBuilder(decoder, accountData)
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

func TestACLStateBuilder_UserJoinBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userjoinexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := createACLStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		keys.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	aId := keychain.GeneratedIdentities["A"]
	bId := keychain.GeneratedIdentities["B"]
	cId := keychain.GeneratedIdentities["C"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, pb.ACLChange_Admin)
	assert.Equal(t, aclState.userStates[bId].Permissions, pb.ACLChange_Writer)
	assert.Equal(t, aclState.userStates[cId].Permissions, pb.ACLChange_Reader)

	var changeIds []string
	ctx.Tree.Iterate(ctx.Tree.root.Id, func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "B.1.2"})
}

func TestACLStateBuilder_UserRemoveBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userremoveexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := createACLStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		keys.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	aId := keychain.GeneratedIdentities["A"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, pb.ACLChange_Admin)

	var changeIds []string
	ctx.Tree.Iterate(ctx.Tree.root.Id, func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "A.1.3", "A.1.4"})
}

func TestACLStateBuilder_UserRemoveBeforeBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userremovebeforeexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := createACLStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		keys.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "C", "E"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))
	assert.Nil(t, aclState.userStates[keychain.GetIdentity("B")])

	var changeIds []string
	ctx.Tree.Iterate(ctx.Tree.root.Id, func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "B.1.1", "A.1.2", "A.1.3"})
}

func TestACLStateBuilder_InvalidSnapshotBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/invalidsnapshotexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := createACLStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		keys.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))

	var changeIds []string
	ctx.Tree.Iterate(ctx.Tree.root.Id, func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, []string{"A.1.1", "B.1.1", "A.1.2", "A.1.3", "B.1.2"}, changeIds)
}

func TestACLStateBuilder_ValidSnapshotBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/validsnapshotexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := createACLStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		keys.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))

	var changeIds []string
	ctx.Tree.Iterate(ctx.Tree.root.Id, func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, []string{"A.1.2", "A.1.3", "B.1.2"}, changeIds)
}
