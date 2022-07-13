package plaintextdocument

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/treestoragebuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDocument_NewPlainTextDocument(t *testing.T) {
	keychain := treestoragebuilder.NewKeychain()
	keychain.AddSigningKey("A")
	keychain.AddEncryptionKey("A")
	data := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  keys.NewEd25519Decoder(),
	}

	doc, err := NewPlainTextDocument(data, treestorage.NewInMemoryTreeStorage, "Some text")
	if err != nil {
		t.Fatalf("should not create document with error: %v", err)
	}
	assert.Equal(t, doc.Text(), "Some text")
}

func TestDocument_PlainTextDocument_AddText(t *testing.T) {
	keychain := treestoragebuilder.NewKeychain()
	keychain.AddSigningKey("A")
	keychain.AddEncryptionKey("A")
	data := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
		Decoder:  keys.NewEd25519Decoder(),
	}

	doc, err := NewPlainTextDocument(data, treestorage.NewInMemoryTreeStorage, "Some text")
	if err != nil {
		t.Fatalf("should not create document with error: %v", err)
	}

	err = doc.AddText("Next")
	if err != nil {
		t.Fatalf("should be able to add document: %v", err)
	}
	assert.Equal(t, doc.Text(), "Some text|Next")

	err = doc.AddText("Shmext")
	if err != nil {
		t.Fatalf("should be able to add document: %v", err)
	}
	assert.Equal(t, doc.Text(), "Some text|Next|Shmext")
}
