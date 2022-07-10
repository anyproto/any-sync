package acltree

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/testutils/threadbuilder"
)

func TestACLTree_UserJoinBuild(t *testing.T) {
	thr, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userjoinexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
	}
	tree, err := BuildACLTree(thr, accountData)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	//fmt.Println(ctx.Tree.Graph())
	aId := keychain.GeneratedIdentities["A"]
	bId := keychain.GeneratedIdentities["B"]
	cId := keychain.GeneratedIdentities["C"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, pb.ACLChange_Admin)
	assert.Equal(t, aclState.userStates[bId].Permissions, pb.ACLChange_Writer)
	assert.Equal(t, aclState.userStates[cId].Permissions, pb.ACLChange_Reader)

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "B.1.2"})
}

func TestACLTree_UserRemoveBuild(t *testing.T) {
	thr, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userremoveexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
	}
	tree, err := BuildACLTree(thr, accountData)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	//fmt.Println(ctx.Tree.Graph())
	aId := keychain.GeneratedIdentities["A"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, pb.ACLChange_Admin)

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "A.1.3", "A.1.4"})
}

func TestACLTree_UserRemoveBeforeBuild(t *testing.T) {
	thr, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userremovebeforeexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
	}
	tree, err := BuildACLTree(thr, accountData)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "C", "E"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))
	assert.Nil(t, aclState.userStates[keychain.GetIdentity("B")])

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "B.1.1", "A.1.2", "A.1.3"})
}

func TestACLTree_InvalidSnapshotBuild(t *testing.T) {
	thr, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/invalidsnapshotexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
	}
	tree, err := BuildACLTree(thr, accountData)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, []string{"A.1.1", "B.1.1", "A.1.2", "A.1.3", "B.1.2"}, changeIds)
}

func TestACLTree_ValidSnapshotBuild(t *testing.T) {
	thr, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/validsnapshotexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
	}
	tree, err := BuildACLTree(thr, accountData)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	//fmt.Println(ctx.Tree.Graph())
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, pb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, []string{"A.1.2", "A.1.3", "B.1.2"}, changeIds)
}
