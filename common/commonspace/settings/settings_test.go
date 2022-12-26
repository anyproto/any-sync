package settings

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice/mock_accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/accountdata"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/treegetter/mock_treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings/deletionstate/mock_deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settings/mock_settings"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage/mock_spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
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
	spaceId      string
	docId        string
	doc          *settingsObject
	ctrl         *gomock.Controller
	treeGetter   *mock_treegetter.MockTreeGetter
	spaceStorage *mock_spacestorage.MockSpaceStorage
	provider     *mock_settings.MockDeletedIdsProvider
	deleter      *mock_settings.MockDeleter
	syncTree     *mock_synctree.MockSyncTree
	delState     *mock_deletionstate.MockDeletionState
	account      *mock_accountservice.MockService
}

func newSettingsFixture(t *testing.T) *settingsFixture {
	spaceId := "spaceId"
	objectId := "objectId"

	ctrl := gomock.NewController(t)
	acc := mock_accountservice.NewMockService(ctrl)
	treeGetter := mock_treegetter.NewMockTreeGetter(ctrl)
	st := mock_spacestorage.NewMockSpaceStorage(ctrl)
	delState := mock_deletionstate.NewMockDeletionState(ctrl)
	prov := mock_settings.NewMockDeletedIdsProvider(ctrl)
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
		prov:          prov,
		del:           del,
	}
	doc := NewSettingsObject(deps, spaceId).(*settingsObject)
	return &settingsFixture{
		spaceId:      spaceId,
		docId:        objectId,
		doc:          doc,
		ctrl:         ctrl,
		treeGetter:   treeGetter,
		spaceStorage: st,
		provider:     prov,
		deleter:      del,
		syncTree:     syncTree,
		account:      acc,
		delState:     delState,
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
	fx.delState.EXPECT().CreateDeleteChange(delId, false).Return(res, nil)

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

	lastChangeId := "someId"
	retIds := []string{"id1", "id2"}
	fx.doc.lastChangeId = lastChangeId
	fx.provider.EXPECT().ProvideIds(gomock.Not(nil), lastChangeId).Return(retIds, retIds[len(retIds)-1], nil)
	fx.delState.EXPECT().Add(retIds).Return(nil)
	err = fx.doc.DeleteObject(delId)
	require.NoError(t, err)
	require.Equal(t, retIds[len(retIds)-1], fx.doc.lastChangeId)

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

	lastChangeId := "someId"
	retIds := []string{"id1", "id2"}
	fx.doc.lastChangeId = lastChangeId
	fx.provider.EXPECT().ProvideIds(gomock.Not(nil), "").Return(retIds, retIds[len(retIds)-1], nil)
	fx.delState.EXPECT().Add(retIds).Return(nil)

	fx.doc.Rebuild(fx.doc)
	require.Equal(t, retIds[len(retIds)-1], fx.doc.lastChangeId)

	fx.syncTree.EXPECT().Close().Return(nil)
	err = fx.doc.Close()
	require.NoError(t, err)
}
