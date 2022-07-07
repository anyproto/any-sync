package data

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadbuilder"
)

func TestDocument_Build(t *testing.T) {
	thread, err := threadbuilder.NewThreadBuilderFromFile("threadbuilder/userjoinexample.yml")
	if err != nil {
		t.Fatal(err)
	}
	keychain := thread.GetKeychain()
	accountData := &AccountData{
		Identity: keychain.GetIdentity("A"),
		EncKey:   keychain.EncryptionKeys["A"],
	}
	doc := NewDocument(thread, NewPlainTextDocumentStateProvider(), accountData)
	res, err := doc.Build()
	if err != nil {
		t.Fatal(err)
	}

	st := res.(*PlainTextDocumentState)
	assert.Equal(t, st.Text, "some text|first")
}
