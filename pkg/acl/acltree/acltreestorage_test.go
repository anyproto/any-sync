package acltree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/treestoragebuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_BuildTreeStorageWithACL(t *testing.T) {
	keychain := treestoragebuilder.NewKeychain()
	keychain.AddSigningKey("A")
	keychain.AddEncryptionKey("A")
	data := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
	}
	thr, err := CreateNewTreeStorageWithACL(
		data,
		func(builder ChangeBuilder) error {
			return builder.UserAdd(
				keychain.GetIdentity("A"),
				keychain.EncryptionKeys["A"].GetPublic(),
				aclpb.ACLChange_Admin)
		},
		treestorage.NewInMemoryTreeStorage)
	if err != nil {
		t.Fatalf("build should not return error")
	}

	heads, err := thr.Heads()
	if err != nil {
		t.Fatalf("should return heads: %v", err)
	}
	if len(heads) == 0 {
		t.Fatalf("tree storage should have non-empty heads")
	}

	header, err := thr.Header()
	if err != nil {
		t.Fatalf("tree storage header should return without error: %v", err)
	}
	assert.Equal(t, heads[0], header.FirstChangeId)

	treeId, err := thr.TreeID()
	if err != nil {
		t.Fatalf("tree id should return without error: %v", err)
	}
	assert.NotEmpty(t, treeId)
	ch, err := thr.GetChange(context.Background(), header.FirstChangeId)
	if err != nil {
		t.Fatalf("get change should not return error: %v", err)
	}

	_, err = NewFromRawChange(ch)
	if err != nil {
		t.Fatalf("we should be able to unmarshall change: %v", err)
	}
}
