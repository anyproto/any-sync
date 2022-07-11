package plaintextdocument

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/testutils/threadbuilder"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDocumentStateBuilder_UserJoinBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userjoinexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := acltree.createDocumentStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		NewPlainTextDocumentStateProvider(),
		threadmodels.NewEd25519Decoder())
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}

	st := ctx.DocState.(*DocumentState)
	assert.Equal(t, st.Text, "some text|first")
}

func TestDocumentStateBuilder_UserRemoveBuild(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userremoveexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	ctx, err := acltree.createDocumentStateFromThread(
		thread,
		keychain.GetIdentity("A"),
		keychain.EncryptionKeys["A"],
		NewPlainTextDocumentStateProvider(),
		threadmodels.NewEd25519Decoder())
	if err != nil {
		t.Fatalf("should build acl aclState without err: %v", err)
	}

	st := ctx.DocState.(*DocumentState)
	assert.Equal(t, st.Text, "some text|first")
}
