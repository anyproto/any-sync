package settingsdocument

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account/mock_account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/deletionstate/mock_deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/mock_settingsdocument"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage/mock_storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/mock_synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter/mock_treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
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
	doc          *settingsDocument
	ctrl         *gomock.Controller
	treeGetter   *mock_treegetter.MockTreeGetter
	spaceStorage *mock_storage.MockSpaceStorage
	provider     *mock_settingsdocument.MockDeletedIdsProvider
	deleter      *mock_settingsdocument.MockDeleter
	syncTree     *mock_synctree.MockSyncTree
	delState     *mock_deletionstate.MockDeletionState
	account      *mock_account.MockService
}

func newSettingsFixture(t *testing.T) *settingsFixture {
	spaceId := "spaceId"
	docId := "documentId"

	ctrl := gomock.NewController(t)
	acc := mock_account.NewMockService(ctrl)
	treeGetter := mock_treegetter.NewMockTreeGetter(ctrl)
	st := mock_storage.NewMockSpaceStorage(ctrl)
	delState := mock_deletionstate.NewMockDeletionState(ctrl)
	prov := mock_settingsdocument.NewMockDeletedIdsProvider(ctrl)
	syncTree := mock_synctree.NewMockSyncTree(ctrl)
	del := mock_settingsdocument.NewMockDeleter(ctrl)

	delState.EXPECT().AddObserver(gomock.Any())

	buildFunc := BuildTreeFunc(func(ctx context.Context, id string, listener updatelistener.UpdateListener) (synctree.SyncTree, error) {
		require.Equal(t, docId, id)
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
	doc := NewSettingsDocument(deps, spaceId).(*settingsDocument)
	return &settingsFixture{
		spaceId:      spaceId,
		docId:        docId,
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

func TestSettingsDocument_Init(t *testing.T) {
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

func TestSettingsDocument_DeleteObject(t *testing.T) {
	fx := newSettingsFixture(t)
	defer fx.stop()

	fx.spaceStorage.EXPECT().SpaceSettingsId().Return(fx.docId)
	fx.deleter.EXPECT().Delete()

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	delId := "delId"

	fx.syncTree.EXPECT().ID().Return("syncId")
	fx.delState.EXPECT().Exists(delId).Return(false)
	fx.spaceStorage.EXPECT().TreeStorage(delId).Return(nil, nil)
	res := []byte("settingsData")
	fx.delState.EXPECT().CreateDeleteChange(delId, false).Return(res, nil)

	accountData := &account.AccountData{
		Identity: []byte("id"),
		PeerKey:  nil,
		SignKey:  &signingkey.Ed25519PrivateKey{},
		EncKey:   nil,
	}
	fx.account.EXPECT().Account().Return(accountData)
	fx.syncTree.EXPECT().AddContent(gomock.Any(), tree.SignableChangeContent{
		Data:        res,
		Key:         accountData.SignKey,
		Identity:    accountData.Identity,
		IsSnapshot:  false,
		IsEncrypted: false,
	}).Return(tree.AddResult{}, nil)

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

func TestSettingsDocument_Rebuild(t *testing.T) {
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
