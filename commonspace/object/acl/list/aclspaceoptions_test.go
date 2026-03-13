package list

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/util/crypto"
)

func newRandomAccountData() (*accountdata.AccountKeys, error) {
	return accountdata.NewRandom()
}

func newTestAclRecordBuilder(keys *accountdata.AccountKeys) AclRecordBuilder {
	return NewAclRecordBuilder("", crypto.NewKeyStorage(), keys, recordverifier.NewValidateFull())
}

func newTestReadKeyChangePayload() ReadKeyChangePayload {
	privKey, _, _ := crypto.GenerateRandomEd25519KeyPair()
	return ReadKeyChangePayload{
		MetadataKey: privKey,
		ReadKey:     crypto.NewAES(),
	}
}

func TestAclSpaceOptions_OwnerCanSetAndChange(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []string{
		"a.init::a",
		"a.space_options::restrict_delete",
	}
	for _, cmd := range cmds {
		err := a.Execute(cmd)
		require.NoError(t, err)
	}

	acl := a.ActualAccounts()["a"].Acl
	opts := acl.AclState().CurrentOptions()
	require.NotNil(t, opts)
	require.True(t, opts.DeleteRestricted)

	// Owner can turn it off
	err := a.Execute("a.space_options::unrestrict_delete")
	require.NoError(t, err)

	opts = acl.AclState().CurrentOptions()
	require.NotNil(t, opts)
	require.False(t, opts.DeleteRestricted)
}

func TestAclSpaceOptions_NonOwnerCannotChange(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []string{
		"a.init::a",
		"a.invite::invId",
		"b.join::invId",
		"a.approve::b,adm",
	}
	for _, cmd := range cmds {
		err := a.Execute(cmd)
		require.NoError(t, err)
	}

	// Admin cannot change space options
	err := a.Execute("b.space_options::restrict_delete")
	require.ErrorIs(t, err, ErrInsufficientPermissions)
}

func TestAclSpaceOptions_WriterCannotChange(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []string{
		"a.init::a",
		"a.invite::invId",
		"b.join::invId",
		"a.approve::b,rw",
	}
	for _, cmd := range cmds {
		err := a.Execute(cmd)
		require.NoError(t, err)
	}

	// Writer cannot change space options
	err := a.Execute("b.space_options::restrict_delete")
	require.ErrorIs(t, err, ErrInsufficientPermissions)
}

func TestAclSpaceOptions_OptionsAtRecord(t *testing.T) {
	a := NewAclExecutor("spaceId")
	cmds := []string{
		"a.init::a",
		"a.invite::invId",
		"b.join::invId",
		"a.approve::b,rw",
	}
	for _, cmd := range cmds {
		err := a.Execute(cmd)
		require.NoError(t, err)
	}

	acl := a.ActualAccounts()["a"].Acl

	// Before setting options, OptionsAtRecord should return nil
	headBeforeRestriction := acl.Head().Id
	opts := acl.AclState().OptionsAtRecord(headBeforeRestriction)
	require.Nil(t, opts)

	// Set restriction
	err := a.Execute("a.space_options::restrict_delete")
	require.NoError(t, err)

	// After restriction, OptionsAtRecord at the new head should return the options
	headAfterRestriction := acl.Head().Id
	st := acl.AclState()
	opts = st.OptionsAtRecord(headAfterRestriction)
	require.NotNil(t, opts)
	require.True(t, opts.DeleteRestricted)

	// OptionsAtRecord at the old head (before restriction) should still be nil
	opts = st.OptionsAtRecord(headBeforeRestriction)
	require.Nil(t, opts)
}

func TestAclSpaceOptions_SetAtRoot(t *testing.T) {
	// Build a root with options set
	keys, err := newRandomAccountData()
	require.NoError(t, err)

	builder := newTestAclRecordBuilder(keys)
	root, err := builder.BuildRoot(RootContent{
		PrivKey:   keys.SignKey,
		MasterKey: keys.SignKey,
		SpaceId:   "spaceId",
		Change:    newTestReadKeyChangePayload(),
		Metadata:  []byte("owner"),
		Options:   &aclrecordproto.AclSpaceOptions{DeleteRestricted: true},
	})
	require.NoError(t, err)

	acl, err := newInMemoryAclWithRoot(keys, root)
	require.NoError(t, err)

	st := acl.AclState()
	opts := st.CurrentOptions()
	require.NotNil(t, opts)
	require.True(t, opts.DeleteRestricted)

	// OptionsAtRecord at root head should return the options
	opts = st.OptionsAtRecord(acl.Head().Id)
	require.NotNil(t, opts)
	require.True(t, opts.DeleteRestricted)
}

func TestAclSpaceOptions_CurrentOptionsNilWhenNotSet(t *testing.T) {
	a := NewAclExecutor("spaceId")
	err := a.Execute("a.init::a")
	require.NoError(t, err)

	st := a.ActualAccounts()["a"].Acl.AclState()
	require.Nil(t, st.CurrentOptions())
}
