package settingsdocument

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account/mock_account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/deletionstate"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/settingsdocument/mock_settingsdocument"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage/mock_storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/mock_synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter/mock_treegetter"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type settingsFixture struct {
	spaceId      string
	docId        string
	doc          SettingsDocument
	ctrl         *gomock.Controller
	treeGetter   *mock_treegetter.MockTreeGetter
	spaceStorage *mock_storage.MockSpaceStorage
	provider     *mock_settingsdocument.MockDeletedIdsProvider
	deleter      *mock_settingsdocument.MockDeleter
	syncTree     *mock_synctree.MockSyncTree
	delState     *deletionstate.DeletionState
	account      *mock_account.MockService
}

func newSettingsFixture(t *testing.T) *settingsFixture {
	spaceId := "spaceId"
	docId := "documentId"

	ctrl := gomock.NewController(t)
	acc := mock_account.NewMockService(ctrl)
	treeGetter := mock_treegetter.NewMockTreeGetter(ctrl)
	st := mock_storage.NewMockSpaceStorage(ctrl)
	delState := deletionstate.NewDeletionState(st)
	prov := mock_settingsdocument.NewMockDeletedIdsProvider(ctrl)
	syncTree := mock_synctree.NewMockSyncTree(ctrl)
	del := mock_settingsdocument.NewMockDeleter(ctrl)

	buildFunc := BuildTreeFunc(func(ctx context.Context, id string, listener updatelistener.UpdateListener) (synctree.SyncTree, error) {
		require.Equal(t, docId, id)
		return syncTree, nil
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
	doc := NewSettingsDocument(deps, spaceId)
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
	fx.syncTree.EXPECT().Close().Return(nil)

	err := fx.doc.Init(context.Background())
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	err = fx.doc.Close()
	require.NoError(t, err)
}
