package synctree

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
)

type syncHandlerFixture struct {
	ctrl        *gomock.Controller
	tree        *testSyncTreeMock
	client      *mock_synctree.MockSyncClient
	syncHandler *syncHandler
}

type testSyncTreeMock struct {
	*mock_synctree.MockSyncTree
}

func newTestSyncTreeMock(obj *mock_synctree.MockSyncTree) *testSyncTreeMock {
	return &testSyncTreeMock{obj}
}

func (t *testSyncTreeMock) Lock() {
}

func (t *testSyncTreeMock) Unlock() {
}

func newSyncHandlerFixture(t *testing.T) *syncHandlerFixture {
	ctrl := gomock.NewController(t)
	tree := newTestSyncTreeMock(mock_synctree.NewMockSyncTree(ctrl))
	client := mock_synctree.NewMockSyncClient(ctrl)
	syncHandler := &syncHandler{
		tree:       tree,
		syncClient: client,
		spaceId:    "spaceId",
	}
	return &syncHandlerFixture{
		ctrl:        ctrl,
		tree:        tree,
		client:      client,
		syncHandler: syncHandler,
	}
}

func (fx *syncHandlerFixture) finish() {
	fx.ctrl.Finish()
}

func