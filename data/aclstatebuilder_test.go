package data

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadbuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
		threadmodels.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
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
	ctx.Tree.iterate(ctx.Tree.root, func(c *Change) (isContinue bool) {
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
		threadmodels.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	aId := keychain.GeneratedIdentities["A"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, pb.ACLChange_Admin)

	var changeIds []string
	ctx.Tree.iterate(ctx.Tree.root, func(c *Change) (isContinue bool) {
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
		threadmodels.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "C", "E"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))
	assert.Nil(t, aclState.userStates[keychain.GetIdentity("B")])

	var changeIds []string
	ctx.Tree.iterate(ctx.Tree.root, func(c *Change) (isContinue bool) {
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
		threadmodels.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))

	var changeIds []string
	ctx.Tree.iterate(ctx.Tree.root, func(c *Change) (isContinue bool) {
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
		threadmodels.NewEd25519Decoder(),
		false)
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}
	aclState := ctx.ACLState
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))

	var changeIds []string
	ctx.Tree.iterate(ctx.Tree.root, func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, []string{"A.1.2", "A.1.3", "B.1.2"}, changeIds)
}
