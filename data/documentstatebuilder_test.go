package data

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadbuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDocumentStateBuilder_UserJoinBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userjoinexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := createDocumentStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		threadmodels.NewEd25519Decoder())
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}

	st := ctx.DocState
	allIds := make(map[string]bool)
	for _, b := range st.Blocks() {
		allIds[b.Id] = true
	}
	if err != nil {
		t.Fatalf("iterate should not return error: %v", err)
	}
	assert.True(t, allIds["root"])
	assert.True(t, allIds["first"])
}

func TestDocumentStateBuilder_UserRemoveBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userremoveexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := createDocumentStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		threadmodels.NewEd25519Decoder())
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}

	st := ctx.DocState
	allIds := make(map[string]bool)
	for _, b := range st.Blocks() {
		allIds[b.Id] = true
	}
	if err != nil {
		t.Fatalf("iterate should not return error: %v", err)
	}
	assert.True(t, allIds["root"])
	assert.True(t, allIds["second"])
}
