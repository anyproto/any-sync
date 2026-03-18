package settings

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/mock_list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func makeDeleteChangeData(t *testing.T, id string) []byte {
	t.Helper()
	data := &spacesyncproto.SettingsData{
		Content: []*spacesyncproto.SpaceSettingsContent{
			{
				Value: &spacesyncproto.SpaceSettingsContent_ObjectDelete{
					ObjectDelete: &spacesyncproto.ObjectDelete{Id: id},
				},
			},
		},
	}
	b, err := data.MarshalVT()
	require.NoError(t, err)
	return b
}

func makeNonDeleteChangeData(t *testing.T) []byte {
	t.Helper()
	data := &spacesyncproto.SettingsData{
		Content: []*spacesyncproto.SpaceSettingsContent{
			{
				Value: &spacesyncproto.SpaceSettingsContent_SpaceDelete{
					SpaceDelete: &spacesyncproto.SpaceDelete{},
				},
			},
		},
	}
	b, err := data.MarshalVT()
	require.NoError(t, err)
	return b
}

type validatorTestSetup struct {
	executor *list.AclTestExecutor
	acl      list.AclList
}

func newValidatorTestSetup(t *testing.T, restricted bool) *validatorTestSetup {
	t.Helper()
	a := list.NewAclExecutor("spaceId")
	cmds := []string{
		"a.init::a",
		"a.invite::inv1",
		"b.join::inv1",
		"a.approve::b,rw",
		"a.invite::inv2",
		"c.join::inv2",
		"a.approve::c,adm",
	}
	for _, cmd := range cmds {
		err := a.Execute(cmd)
		require.NoError(t, err)
	}
	if restricted {
		err := a.Execute("a.space_options::restrict_delete")
		require.NoError(t, err)
	}
	return &validatorTestSetup{
		executor: a,
		acl:      a.ActualAccounts()["a"].Acl,
	}
}

func (s *validatorTestSetup) identityOf(account string) crypto.PubKey {
	return s.executor.ActualAccounts()[account].Keys.SignKey.GetPublic()
}

// noopGetAuthor is used for tests where the author lookup is not expected to be called.
func noopGetAuthor(_ string) (crypto.PubKey, error) {
	return nil, errors.New("unexpected call to getAuthor")
}

func TestSettingsContentValidator_NoRestriction_WriterCanDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, false)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	validator := newSettingsContentValidator(noopGetAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "obj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    setup.identityOf("b"), // writer
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_Restricted_WriterCannotDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	// getAuthor returns a different identity (owner "a"), so writer "b" is not the author
	getAuthor := func(_ string) (crypto.PubKey, error) {
		return setup.identityOf("a"), nil
	}
	validator := newSettingsContentValidator(getAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "obj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    setup.identityOf("b"), // writer
	}

	err := validator(change, mockAcl)
	require.ErrorIs(t, err, list.ErrInsufficientPermissions)
}

func TestSettingsContentValidator_Restricted_AdminCanDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	validator := newSettingsContentValidator(noopGetAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "obj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    setup.identityOf("c"), // admin
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_Restricted_OwnerCanDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	validator := newSettingsContentValidator(noopGetAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "obj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    setup.identityOf("a"), // owner
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_NonDeleteChange_PassesEvenWhenRestricted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	validator := newSettingsContentValidator(noopGetAuthor)
	change := &objecttree.Change{
		Data:        makeNonDeleteChangeData(t),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    setup.identityOf("b"), // writer
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_RootChange_Skipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAcl := mock_list.NewMockAclList(ctrl)

	validator := newSettingsContentValidator(noopGetAuthor)
	change := &objecttree.Change{
		Data: makeDeleteChangeData(t, "obj1"),
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_NilData_Skipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAcl := mock_list.NewMockAclList(ctrl)

	validator := newSettingsContentValidator(noopGetAuthor)
	change := &objecttree.Change{
		PreviousIds: []string{"prev"},
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_Restricted_AuthorCanDeleteOwnObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	writerIdentity := setup.identityOf("b")
	// getAuthor returns writer "b"'s identity — they are the author
	getAuthor := func(_ string) (crypto.PubKey, error) {
		return writerIdentity, nil
	}
	validator := newSettingsContentValidator(getAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "obj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    writerIdentity,
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_Restricted_NonAuthorWriterCannotDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	// Object was authored by owner "a", but writer "b" tries to delete
	getAuthor := func(_ string) (crypto.PubKey, error) {
		return setup.identityOf("a"), nil
	}
	validator := newSettingsContentValidator(getAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "obj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    setup.identityOf("b"), // writer, not the author
	}

	err := validator(change, mockAcl)
	require.ErrorIs(t, err, list.ErrInsufficientPermissions)
}

func TestSettingsContentValidator_Restricted_AuthorCheckWithParentResolution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	writerIdentity := setup.identityOf("b")
	// Simulate parent resolution: getAuthor is called with child object ID
	// and returns the parent's author which matches the change author
	getAuthor := func(objId string) (crypto.PubKey, error) {
		require.Equal(t, "childObj1", objId)
		return writerIdentity, nil
	}
	validator := newSettingsContentValidator(getAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "childObj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    writerIdentity,
	}

	err := validator(change, mockAcl)
	require.NoError(t, err)
}

func TestSettingsContentValidator_Restricted_AuthorLookupError_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setup := newValidatorTestSetup(t, true)
	mockAcl := mock_list.NewMockAclList(ctrl)
	mockAcl.EXPECT().AclState().Return(setup.acl.AclState()).AnyTimes()

	getAuthor := func(_ string) (crypto.PubKey, error) {
		return nil, errors.New("storage error")
	}
	validator := newSettingsContentValidator(getAuthor)
	change := &objecttree.Change{
		Data:        makeDeleteChangeData(t, "obj1"),
		PreviousIds: []string{"prev"},
		AclHeadId:   setup.acl.Head().Id,
		Identity:    setup.identityOf("b"),
	}

	err := validator(change, mockAcl)
	require.ErrorIs(t, err, list.ErrInsufficientPermissions)
}
