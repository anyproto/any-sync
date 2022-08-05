package acltree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/treestoragebuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockListener struct{}

func (m *mockListener) Update(tree ACLTree) {}

func (m *mockListener) Rebuild(tree ACLTree) {}

func TestACLTree_UserJoinBuild(t *testing.T) {
	thr, err := treestoragebuilder.NewTreeStorageBuilderWithTestName("userjoinexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}
	listener := &mockListener{}
	tree, err := BuildACLTree(thr, accountData, listener)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	aId := keychain.GeneratedIdentities["A"]
	bId := keychain.GeneratedIdentities["B"]
	cId := keychain.GeneratedIdentities["C"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, aclpb.ACLChange_Admin)
	assert.Equal(t, aclState.userStates[bId].Permissions, aclpb.ACLChange_Writer)
	assert.Equal(t, aclState.userStates[cId].Permissions, aclpb.ACLChange_Reader)

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "B.1.2"})
}

func TestACLTree_UserJoinUpdate_Append(t *testing.T) {
	thr, err := treestoragebuilder.NewTreeStorageBuilderWithTestName("userjoinexampleupdate.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}

	listener := &mockListener{}
	tree, err := BuildACLTree(thr, accountData, listener)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	rawChanges := thr.GetUpdates("append")
	res, err := tree.AddRawChanges(context.Background(), rawChanges...)
	assert.Equal(t, res.Summary, AddResultSummaryAppend)

	aclState := tree.ACLState()
	aId := keychain.GeneratedIdentities["A"]
	bId := keychain.GeneratedIdentities["B"]
	cId := keychain.GeneratedIdentities["C"]
	dId := keychain.GeneratedIdentities["D"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, aclpb.ACLChange_Admin)
	assert.Equal(t, aclState.userStates[bId].Permissions, aclpb.ACLChange_Writer)
	assert.Equal(t, aclState.userStates[cId].Permissions, aclpb.ACLChange_Reader)
	assert.Equal(t, aclState.userStates[dId].Permissions, aclpb.ACLChange_Writer)

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "B.1.2", "B.1.3", "A.1.4"})
}

func TestACLTree_UserJoinUpdate_Rebuild(t *testing.T) {
	thr, err := treestoragebuilder.NewTreeStorageBuilderWithTestName("userjoinexampleupdate.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}
	listener := &mockListener{}
	tree, err := BuildACLTree(thr, accountData, listener)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	rawChanges := thr.GetUpdates("rebuild")
	res, err := tree.AddRawChanges(context.Background(), rawChanges...)
	assert.Equal(t, res.Summary, AddResultSummaryRebuild)

	aclState := tree.ACLState()
	aId := keychain.GeneratedIdentities["A"]
	bId := keychain.GeneratedIdentities["B"]
	cId := keychain.GeneratedIdentities["C"]
	dId := keychain.GeneratedIdentities["D"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, aclpb.ACLChange_Admin)
	assert.Equal(t, aclState.userStates[bId].Permissions, aclpb.ACLChange_Writer)
	assert.Equal(t, aclState.userStates[cId].Permissions, aclpb.ACLChange_Reader)
	assert.Equal(t, aclState.userStates[dId].Permissions, aclpb.ACLChange_Writer)

	var changeIds []string

	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "B.1.2", "A.1.4"})
}

func TestACLTree_UserRemoveBuild(t *testing.T) {
	thr, err := treestoragebuilder.NewTreeStorageBuilderWithTestName("userremoveexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}
	listener := &mockListener{}
	tree, err := BuildACLTree(thr, accountData, listener)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	aId := keychain.GeneratedIdentities["A"]

	assert.Equal(t, aclState.identity, aId)
	assert.Equal(t, aclState.userStates[aId].Permissions, aclpb.ACLChange_Admin)

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, changeIds, []string{"A.1.1", "A.1.2", "B.1.1", "A.1.3", "A.1.4"})
}

func TestACLTree_UserRemoveBeforeBuild(t *testing.T) {
	thr, err := treestoragebuilder.NewTreeStorageBuilderWithTestName("userremovebeforeexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}
	listener := &mockListener{}
	tree, err := BuildACLTree(thr, accountData, listener)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	for _, s := range []string{"A", "C", "E"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, aclpb.ACLChange_Admin)
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
	thr, err := treestoragebuilder.NewTreeStorageBuilderWithTestName("invalidsnapshotexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}
	listener := &mockListener{}
	tree, err := BuildACLTree(thr, accountData, listener)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, aclpb.ACLChange_Admin)
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
	thr, err := treestoragebuilder.NewTreeStorageBuilderWithTestName("validsnapshotexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thr.GetKeychain()
	accountData := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  signingkey.NewEd25519PubKeyDecoder(),
	}
	listener := &mockListener{}
	tree, err := BuildACLTree(thr, accountData, listener)
	if err != nil {
		t.Fatalf("should Build acl ACLState without err: %v", err)
	}
	aclState := tree.ACLState()
	for _, s := range []string{"A", "B", "C", "D", "E", "F"} {
		assert.Equal(t, aclState.userStates[keychain.GetIdentity(s)].Permissions, aclpb.ACLChange_Admin)
	}
	assert.Equal(t, aclState.identity, keychain.GetIdentity("A"))

	var changeIds []string
	tree.Iterate(func(c *Change) (isContinue bool) {
		changeIds = append(changeIds, c.Id)
		return true
	})
	assert.Equal(t, []string{"A.1.2", "A.1.3", "B.1.2"}, changeIds)
}
