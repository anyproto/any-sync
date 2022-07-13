package acltree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/treestoragebuilder"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildThreadWithACL(t *testing.T) {
	keychain := treestoragebuilder.NewKeychain()
	keychain.AddSigningKey("A")
	keychain.AddEncryptionKey("A")
	data := &account.AccountData{
		Identity: keychain.GetIdentity("A"),
		SignKey:  keychain.SigningKeys["A"],
		EncKey:   keychain.EncryptionKeys["A"],
	}
	thr, err := BuildTreeStorageWithACL(
		data,
		func(builder ChangeBuilder) error {
			return builder.UserAdd(
				keychain.GetIdentity("A"),
				keychain.EncryptionKeys["A"].GetPublic(),
				pb.ACLChange_Admin)
		},
		treestorage.NewInMemoryTreeStorage)
	if err != nil {
		t.Fatalf("build should not return error")
	}
	if len(thr.Heads()) == 0 {
		t.Fatalf("thread should have non-empty heads")
	}
	if thr.Header() == nil {
		t.Fatalf("thread should have non-empty header")
	}
	assert.Equal(t, thr.Heads()[0], thr.Header().FirstChangeId)
	assert.NotEmpty(t, thr.TreeID())
	ch, err := thr.GetChange(context.Background(), thr.Header().FirstChangeId)
	if err != nil {
		t.Fatalf("get change should not return error: %v", err)
	}

	_, err = NewFromRawChange(ch)
	if err != nil {
		t.Fatalf("we should be able to unmarshall change: %v", err)
	}
}
