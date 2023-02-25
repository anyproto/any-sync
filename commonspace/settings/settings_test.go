package settings

import (
	"context"
	"github.com/anytypeio/any-sync/accountservice/mock_accountservice"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter/mock_treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/mock_settings"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	"github.com/anytypeio/any-sync/commonspace/settings/settingsstate/mock_settingsstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
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
	treeGetter      *mock_treegetter.MockTreeGetter
	spaceStorage    *mock_spacestorage.MockSpaceStorage
	stateBuilder    *mock_settingsstate.MockStateBuilder
	deletionManager *mock_settings.MockDeletionManager
	changeFactory   *mock_settingsstate.MockChangeFactory
	deleter         *mock_settings.MockDeleter
	syncTree        *mock_synctree.MockSyncTree
	delState        *mock_settingsstate.MockObjectDeletionState
	account         *mock_accountservice.MockService
}

func newSettingsFixture(t *testing.T) *settingsFixture {
	spaceId := "spaceId"
	objectId := "objectId"

	ctrl := gomock.NewController(t)
	acc := mock_accountservice.NewMockService(ctrl)
	treeGetter := mock_treegetter.NewMockTreeGetter(ctrl)
	st := mock_spacestorage.NewMockSpaceStorage(ctrl)
	delState := mock_settingsstate.NewMockObjectDeletionState(ctrl)
	delManager := mock_settings.NewMockDeletionManager(ctrl)
	stateBuilder := mock_settingsstate.NewMockStateBuilder(ctrl)
	changeFactory := mock_settingsstate.NewMockChangeFactory(ctrl)
	syncTree := mock_synctree.NewMockSyncTree(ctrl)
	del := mock_settings.NewMockDeleter(ctrl)

	delState.EXPECT().AddObserver(gomock.Any())

	buildFunc := BuildTreeFunc(func(ctx context.Context, id string, listener updatelistener.UpdateListener) (synctree.SyncTree, error) {
		require.Equal(t, objectId, id)
		return newTestObjMock(syncTree), nil
	})

	deps := Deps{
		BuildFunc:     buildFunc,
		Account:       acc,
		TreeGetter:    treeGetter,
		Store:         st,
		DeletionState: delState,
		delManager:    delManager,
		changeFactory: changeFactory,
		builder:       stateBuilder,
		del:           del,
	}
	doc := NewSettingsObject(deps, spaceId).(*settingsObject)
	return &settingsFixture{
		spaceId:         spaceId,
		docId:           objectId,
		doc:             doc,
		ctrl:            ctrl,
		treeGetter:      treeGetter,
		spaceStorage:    st,
		stateBuilder:    stateBuilder,
		changeFactory:   changeFactory,
		deletionManager: delManager,
		deleter:         del,
		syncTree:        syncTree,
		account:         acc,
		delState:        delState,
	}
}

func (fx *settingsFixture) stop() {
	fx.ctrl.Finish()
}

func TestSettingsObject_Init(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop()

	fx.spaceStorage.EXPECT().SpaceSettingsId().Return(fx.docId)
	fx.deleter.EXPECT().Delete()
	fx.syncTree.EXPECT().Close().Return(nil)

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	err = fx.doc.Close()
	require.NoError(t, err)
}

func TestSettingsObject_DeleteObject(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop()

	fx.spaceStorage.EXPECT().SpaceSettingsId().Return(fx.docId)
	fx.deleter.EXPECT().Delete()

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	delId := "delId"

	fx.syncTree.EXPECT().Id().Return("syncId")
	fx.delState.EXPECT().Exists(delId).Return(false)
	fx.spaceStorage.EXPECT().TreeStorage(delId).Return(nil, nil)
	res := []byte("settingsData")
	fx.doc.state = &settingsstate.State{LastIteratedId: "someId"}
	fx.changeFactory.EXPECT().CreateObjectDeleteChange(delId, fx.doc.state, false).Return(res, nil)

	accountData := &accountdata.AccountData{
		Identity: []byte("id"),
		PeerKey:  nil,
		SignKey:  &signingkey.Ed25519PrivateKey{},
		EncKey:   nil,
	}
	fx.account.EXPECT().Account().Return(accountData)
	fx.syncTree.EXPECT().AddContent(gomock.Any(), objecttree.SignableChangeContent{
		Data:        res,
		Key:         accountData.SignKey,
		Identity:    accountData.Identity,
		IsSnapshot:  false,
		IsEncrypted: false,
	}).Return(objecttree.AddResult{}, nil)

	fx.stateBuilder.EXPECT().Build(fx.doc, fx.doc.state).Return(fx.doc.state, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), fx.doc.state).Return(nil)
	err = fx.doc.DeleteObject(delId)
	require.NoError(t, err)

	fx.syncTree.EXPECT().Close().Return(nil)
	err = fx.doc.Close()
	require.NoError(t, err)
}

func TestSettingsObject_Rebuild(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop()

	fx.spaceStorage.EXPECT().SpaceSettingsId().Return(fx.docId)
	fx.deleter.EXPECT().Delete()

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	newSt := &settingsstate.State{}
	fx.doc.state = &settingsstate.State{}
	fx.stateBuilder.EXPECT().Build(fx.doc, nil).Return(newSt, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), newSt).Return(nil)

	fx.doc.Rebuild(fx.doc)
}

func TestSettingsObject_Update(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop()

	fx.spaceStorage.EXPECT().SpaceSettingsId().Return(fx.docId)
	fx.deleter.EXPECT().Delete()

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	fx.doc.state = &settingsstate.State{}
	fx.stateBuilder.EXPECT().Build(fx.doc, fx.doc.state).Return(fx.doc.state, nil)
	fx.deletionManager.EXPECT().UpdateState(gomock.Any(), fx.doc.state).Return(nil)

	fx.doc.Update(fx.doc)
}

func TestSettingsObject_DeleteSpace(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop()

	fx.spaceStorage.EXPECT().SpaceSettingsId().Return(fx.docId)
	fx.deleter.EXPECT().Delete()

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	deleterId := "delId"
	rawCh := &treechangeproto.RawTreeChangeWithId{
		RawChange: []byte{1},
		Id:        "id",
	}
	changeFactory := settingsstate.NewChangeFactory()
	delChange, _ := changeFactory.CreateSpaceDeleteChange(deleterId, &settingsstate.State{}, false)

	fx.syncTree.EXPECT().UnpackChange(rawCh).Return(delChange, nil)
	fx.syncTree.EXPECT().AddRawChanges(gomock.Any(), objecttree.RawChangesPayload{
		NewHeads:   []string{rawCh.Id},
		RawChanges: []*treechangeproto.RawTreeChangeWithId{rawCh},
	}).Return(objecttree.AddResult{
		Heads: []string{rawCh.Id},
	}, nil)

	err = fx.doc.DeleteSpace(context.Background(), deleterId, rawCh)
	require.NoError(t, err)
}

func TestSettingsObject_DeleteSpaceIncorrectPeerId(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop()

	fx.spaceStorage.EXPECT().SpaceSettingsId().Return(fx.docId)
	fx.deleter.EXPECT().Delete()

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	deleterId := "delId"
	rawCh := &treechangeproto.RawTreeChangeWithId{
		RawChange: []byte{1},
		Id:        "id",
	}
	changeFactory := settingsstate.NewChangeFactory()
	delChange, _ := changeFactory.CreateSpaceDeleteChange("otherId", &settingsstate.State{}, false)

	fx.syncTree.EXPECT().UnpackChange(rawCh).Return(delChange, nil)
	err = fx.doc.DeleteSpace(context.Background(), deleterId, rawCh)
	require.Equal(t, err, ErrIncorrectDeleteChange)
}
