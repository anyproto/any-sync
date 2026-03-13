package settings

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/accountservice/mock_accountservice"
	"github.com/anyproto/any-sync/commonspace/deletionmanager/mock_deletionmanager"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage/mock_headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/statestorage"
	"github.com/anyproto/any-sync/commonspace/headsync/statestorage/mock_statestorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/mock_list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/treemanager/mock_treemanager"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	"github.com/anyproto/any-sync/commonspace/settings/settingsstate/mock_settingsstate"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/anyproto/any-sync/util/crypto"
)

type testSyncTreeMock struct {
	*mock_synctree.MockSyncTree
	m sync.Mutex
}

func newTestObjMock(mockTree *mock_synctree.MockSyncTree) *testSyncTreeMock {
	return &testSyncTreeMock{
		MockSyncTree: mockTree,
	}
}

func (t *testSyncTreeMock) Lock() {
	t.m.Lock()
}

func (t *testSyncTreeMock) Unlock() {
	t.m.Unlock()
}

type settingsFixture struct {
	spaceId         string
	docId           string
	doc             *settingsObject
	ctrl            *gomock.Controller
	treeManager     *mock_treemanager.MockTreeManager
	spaceStorage    *mock_spacestorage.MockSpaceStorage
	stateBuilder    *mock_settingsstate.MockStateBuilder
	deletionManager *mock_deletionmanager.MockDeletionManager
	changeFactory   *mock_settingsstate.MockChangeFactory
	syncTree        *mock_synctree.MockSyncTree
	historyTree     *mock_objecttree.MockObjectTree
	account         *mock_accountservice.MockService
	stateStorage    *mock_statestorage.MockStateStorage
	headStorage     *mock_headstorage.MockHeadStorage
	aclList         *mock_list.MockAclList
}

var ctx = context.Background()

func newSettingsFixture(t *testing.T) *settingsFixture {
	spaceId := "spaceId"
	objectId := "objectId"

	ctrl := gomock.NewController(t)
	acc := mock_accountservice.NewMockService(ctrl)
	treeManager := mock_treemanager.NewMockTreeManager(ctrl)
	st := mock_spacestorage.NewMockSpaceStorage(ctrl)
	delManager := mock_deletionmanager.NewMockDeletionManager(ctrl)
	stateBuilder := mock_settingsstate.NewMockStateBuilder(ctrl)
	changeFactory := mock_settingsstate.NewMockChangeFactory(ctrl)
	syncTree := mock_synctree.NewMockSyncTree(ctrl)
	historyTree := mock_objecttree.NewMockObjectTree(ctrl)
	stateStorage := mock_statestorage.NewMockStateStorage(ctrl)
	headStorage := mock_headstorage.NewMockHeadStorage(ctrl)
	aclList := mock_list.NewMockAclList(ctrl)

	buildFunc := BuildTreeFunc(func(ctx context.Context, id string, listener updatelistener.UpdateListener) (synctree.SyncTree, error) {
		require.Equal(t, objectId, id)
		return newTestObjMock(syncTree), nil
	})
	buildHistoryTree = func(objTree objecttree.ObjectTree) (objecttree.ReadableObjectTree, error) {
		return historyTree, nil
	}

	deps := Deps{
		BuildFunc:     buildFunc,
		Account:       acc,
		TreeManager:   treeManager,
		Store:         st,
		DelManager:    delManager,
		changeFactory: changeFactory,
		builder:       stateBuilder,
	}
	doc := NewSettingsObject(deps, spaceId).(*settingsObject)
	return &settingsFixture{
		spaceId:         spaceId,
		docId:           objectId,
		doc:             doc,
		ctrl:            ctrl,
		treeManager:     treeManager,
		spaceStorage:    st,
		stateBuilder:    stateBuilder,
		changeFactory:   changeFactory,
		deletionManager: delManager,
		syncTree:        syncTree,
		account:         acc,
		historyTree:     historyTree,
		stateStorage:    stateStorage,
		headStorage:     headStorage,
		aclList:         aclList,
	}
}

func (fx *settingsFixture) expectAclNoRestriction() {
	aclState := &list.AclState{} // empty state, CurrentOptions() returns nil
	fx.syncTree.EXPECT().AclList().Return(fx.aclList)
	fx.aclList.EXPECT().RLock()
	fx.aclList.EXPECT().AclState().Return(aclState).AnyTimes()
	fx.aclList.EXPECT().RUnlock()
}

func (fx *settingsFixture) init(t *testing.T) {
	fx.spaceStorage.EXPECT().StateStorage().AnyTimes().Return(fx.stateStorage)
	fx.stateStorage.EXPECT().GetState(gomock.Any()).Return(statestorage.State{
		SettingsId: fx.docId,
	}, nil)
	fx.spaceStorage.EXPECT().HeadStorage().AnyTimes().Return(fx.headStorage)
	fx.stateBuilder.EXPECT().Build(fx.historyTree, nil).Return(&settingsstate.State{}, nil)
	fx.doc.state = &settingsstate.State{}

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
}

func (fx *settingsFixture) stop(t *testing.T) {
	fx.syncTree.EXPECT().Close().Return(nil)

	err := fx.doc.Close()
	require.NoError(t, err)
	fx.ctrl.Finish()
}

func TestSettingsObject_Init(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)

	fx.init(t)
}

func TestSettingsObject_DeleteObject_NoSnapshot(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)
	fx.init(t)
	delId := "delId"
	DoSnapshot = func(len int) bool {
		return false
	}
	fx.syncTree.EXPECT().Id().Return("syncId")
	fx.syncTree.EXPECT().Len().Return(10)
	fx.headStorage.EXPECT().GetEntry(gomock.Any(), gomock.Any()).Return(headstorage.HeadsEntry{
		IsDerived: false,
	}, nil)
	fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), delId).Return(nil, nil)
	fx.expectAclNoRestriction()
	res := []byte("settingsData")
	fx.doc.state = &settingsstate.State{LastIteratedId: "someId"}
	fx.changeFactory.EXPECT().CreateObjectDeleteChange([]string{delId}, fx.doc.state, false).Return(res, nil)

	accountData, err := accountdata.NewRandom()
	require.NoError(t, err)
	fx.account.EXPECT().Account().Return(accountData)
	fx.syncTree.EXPECT().AddContent(gomock.Any(), objecttree.SignableChangeContent{
		Data:              res,
		Key:               accountData.SignKey,
		IsSnapshot:        false,
		ShouldBeEncrypted: false,
	}).Return(objecttree.AddResult{}, nil)

	fx.stateBuilder.EXPECT().Build(fx.doc, fx.doc.state).Return(fx.doc.state, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), fx.doc.state).Return(nil)
	err = fx.doc.DeleteObject(ctx, delId)
	require.NoError(t, err)
}

func TestSettingsObject_DeleteObject_WithSnapshot(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)
	fx.init(t)
	delId := "delId"
	DoSnapshot = func(len int) bool {
		return true
	}
	fx.syncTree.EXPECT().Id().Return("syncId")
	fx.syncTree.EXPECT().Len().Return(10)
	fx.headStorage.EXPECT().GetEntry(gomock.Any(), gomock.Any()).Return(headstorage.HeadsEntry{
		IsDerived: false,
	}, nil)
	fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), delId).Return(nil, nil)
	fx.expectAclNoRestriction()
	res := []byte("settingsData")
	fx.doc.state = &settingsstate.State{LastIteratedId: "someId"}
	fx.changeFactory.EXPECT().CreateObjectDeleteChange([]string{delId}, fx.doc.state, true).Return(res, nil)

	accountData, err := accountdata.NewRandom()
	require.NoError(t, err)
	fx.account.EXPECT().Account().Return(accountData)
	fx.syncTree.EXPECT().AddContent(gomock.Any(), objecttree.SignableChangeContent{
		Data:              res,
		Key:               accountData.SignKey,
		IsSnapshot:        true,
		ShouldBeEncrypted: false,
	}).Return(objecttree.AddResult{}, nil)

	fx.stateBuilder.EXPECT().Build(fx.doc, fx.doc.state).Return(fx.doc.state, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), fx.doc.state).Return(nil)
	err = fx.doc.DeleteObject(ctx, delId)
	require.NoError(t, err)
}

func TestSettingsObject_DeleteDerivedObject(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)
	fx.init(t)
	delId := "delId"
	DoSnapshot = func(len int) bool {
		return false
	}
	fx.syncTree.EXPECT().Id().Return("syncId")
	fx.headStorage.EXPECT().GetEntry(gomock.Any(), gomock.Any()).Return(headstorage.HeadsEntry{
		IsDerived: true,
	}, nil)
	fx.doc.state = &settingsstate.State{LastIteratedId: "someId"}
	err := fx.doc.DeleteObject(ctx, delId)
	require.Equal(t, ErrCantDeleteDerivedObject, err)
}

func TestSettingsObject_Rebuild(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)

	fx.init(t)
	time.Sleep(100 * time.Millisecond)

	newSt := &settingsstate.State{}
	fx.doc.state = &settingsstate.State{}
	fx.stateBuilder.EXPECT().Build(fx.doc, nil).Return(newSt, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), newSt).Return(nil)

	fx.doc.Rebuild(fx.doc)
}

func TestSettingsObject_Update(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)

	fx.init(t)
	time.Sleep(100 * time.Millisecond)

	fx.doc.state = &settingsstate.State{}
	fx.stateBuilder.EXPECT().Build(fx.doc, fx.doc.state).Return(fx.doc.state, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), fx.doc.state).Return(nil)

	fx.doc.Update(fx.doc)
}

func makeRootChangeBytes(t *testing.T, identity crypto.PubKey) []byte {
	t.Helper()
	identityProto, err := identity.Marshall()
	require.NoError(t, err)
	rootChange := &treechangeproto.RootChange{
		Identity: identityProto,
	}
	payload, err := rootChange.MarshalVT()
	require.NoError(t, err)
	rawChange := &treechangeproto.RawTreeChange{
		Payload: payload,
	}
	rawBytes, err := rawChange.MarshalVT()
	require.NoError(t, err)
	return rawBytes
}

func TestSettingsObject_DeleteObject_Restricted_AuthorCanDelete(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)
	fx.init(t)

	delId := "delId"
	DoSnapshot = func(len int) bool {
		return false
	}

	// Use AclTestExecutor to get a real ACL state with restricted delete
	aclSetup := newValidatorTestSetup(t, true)
	aclState := aclSetup.acl.AclState()
	// Use writer "b"'s keys as the current account
	writerKeys := aclSetup.executor.ActualAccounts()["b"].Keys

	fx.syncTree.EXPECT().Id().Return("syncId")
	fx.syncTree.EXPECT().Len().Return(10)
	// GetEntry is called twice: once in DeleteObject, once in objectAuthor
	fx.headStorage.EXPECT().GetEntry(gomock.Any(), delId).Return(headstorage.HeadsEntry{
		Id:        delId,
		IsDerived: false,
	}, nil).Times(2)
	fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), delId).Return(nil, nil)

	// ACL restriction check: writer "b" does not have admin perms
	fx.syncTree.EXPECT().AclList().Return(fx.aclList)
	fx.aclList.EXPECT().RLock()
	fx.aclList.EXPECT().AclState().Return(aclState).AnyTimes()
	fx.aclList.EXPECT().RUnlock()
	fx.account.EXPECT().Account().Return(writerKeys)

	// Author check: objectAuthor resolves to writer "b" (the current user is the author)
	mockTreeStorage := mock_objecttree.NewMockStorage(fx.ctrl)
	fx.spaceStorage.EXPECT().TreeStorage(gomock.Any(), delId).Return(mockTreeStorage, nil)
	mockTreeStorage.EXPECT().Root(gomock.Any()).Return(objecttree.StorageChange{
		RawChange: makeRootChangeBytes(t, writerKeys.SignKey.GetPublic()),
		Id:        "rootId",
	}, nil)
	fx.account.EXPECT().Account().Return(writerKeys)

	// Proceed to create the change
	res := []byte("settingsData")
	fx.doc.state = &settingsstate.State{LastIteratedId: "someId"}
	fx.changeFactory.EXPECT().CreateObjectDeleteChange([]string{delId}, fx.doc.state, false).Return(res, nil)
	fx.account.EXPECT().Account().Return(writerKeys)
	fx.syncTree.EXPECT().AddContent(gomock.Any(), objecttree.SignableChangeContent{
		Data:              res,
		Key:               writerKeys.SignKey,
		IsSnapshot:        false,
		ShouldBeEncrypted: false,
	}).Return(objecttree.AddResult{}, nil)
	fx.stateBuilder.EXPECT().Build(fx.doc, fx.doc.state).Return(fx.doc.state, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), fx.doc.state).Return(nil)

	err := fx.doc.DeleteObject(ctx, delId)
	require.NoError(t, err)
}

func TestSettingsObject_DeleteObject_Restricted_NonAuthorCannotDelete(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop(t)
	fx.init(t)

	delId := "delId"
	DoSnapshot = func(len int) bool {
		return false
	}

	// Use AclTestExecutor to get a real ACL state with restricted delete
	aclSetup := newValidatorTestSetup(t, true)
	aclState := aclSetup.acl.AclState()
	// Use writer "b"'s keys as the current account
	writerKeys := aclSetup.executor.ActualAccounts()["b"].Keys
	// Object was authored by owner "a"
	ownerKeys := aclSetup.executor.ActualAccounts()["a"].Keys

	fx.syncTree.EXPECT().Id().Return("syncId")
	// GetEntry is called twice: once in DeleteObject, once in objectAuthor
	fx.headStorage.EXPECT().GetEntry(gomock.Any(), delId).Return(headstorage.HeadsEntry{
		Id:        delId,
		IsDerived: false,
	}, nil).Times(2)
	fx.headStorage.EXPECT().GetEntriesByParentId(gomock.Any(), delId).Return(nil, nil)

	// ACL restriction check
	fx.syncTree.EXPECT().AclList().Return(fx.aclList)
	fx.aclList.EXPECT().RLock()
	fx.aclList.EXPECT().AclState().Return(aclState).AnyTimes()
	fx.aclList.EXPECT().RUnlock()
	fx.account.EXPECT().Account().Return(writerKeys)

	// Author check: object authored by owner "a", current user is writer "b" → mismatch
	mockTreeStorage := mock_objecttree.NewMockStorage(fx.ctrl)
	fx.spaceStorage.EXPECT().TreeStorage(gomock.Any(), delId).Return(mockTreeStorage, nil)
	mockTreeStorage.EXPECT().Root(gomock.Any()).Return(objecttree.StorageChange{
		RawChange: makeRootChangeBytes(t, ownerKeys.SignKey.GetPublic()),
		Id:        "rootId",
	}, nil)
	fx.account.EXPECT().Account().Return(writerKeys)

	fx.doc.state = &settingsstate.State{LastIteratedId: "someId"}
	err := fx.doc.DeleteObject(ctx, delId)
	require.ErrorIs(t, err, list.ErrInsufficientPermissions)
}
