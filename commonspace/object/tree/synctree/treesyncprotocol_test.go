package synctree

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
)

type treeSyncProtocolFixture struct {
	log            logger.CtxLogger
	spaceId        string
	senderId       string
	treeId         string
	objectTreeMock *testObjTreeMock
	reqFactory     *mock_synctree.MockRequestFactory
	ctrl           *gomock.Controller
	syncProtocol   TreeSyncProtocol
}

func newSyncProtocolFixture(t *testing.T) *treeSyncProtocolFixture {
	ctrl := gomock.NewController(t)
	objTree := &testObjTreeMock{
		MockObjectTree: mock_objecttree.NewMockObjectTree(ctrl),
	}
	spaceId := "spaceId"
	reqFactory := mock_synctree.NewMockRequestFactory(ctrl)
	objTree.EXPECT().Id().Return("treeId")
	syncProtocol := newTreeSyncProtocol(spaceId, objTree, reqFactory)
	return &treeSyncProtocolFixture{
		log:            log,
		spaceId:        spaceId,
		senderId:       "senderId",
		treeId:         "treeId",
		objectTreeMock: objTree,
		reqFactory:     reqFactory,
		ctrl:           ctrl,
		syncProtocol:   syncProtocol,
	}
}

func (fx *treeSyncProtocolFixture) stop() {
	fx.ctrl.Finish()
}

func TestTreeSyncProtocol_HeadUpdate(t *testing.T) {
	ctx := context.Background()
	fullRequest := &treechangeproto.TreeSyncMessage{
		Content: &treechangeproto.TreeSyncContentValue{
			Value: &treechangeproto.TreeSyncContentValue_FullSyncRequest{
				FullSyncRequest: &treechangeproto.TreeFullSyncRequest{},
			},
		},
	}

	t.Run("head update non empty all heads added", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).AnyTimes()
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).Return(true)

		res, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.NoError(t, err)
		require.Nil(t, res)
	})

	t.Run("head update non empty equal heads", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h1"}).AnyTimes()
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)

		res, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.NoError(t, err)
		require.Nil(t, res)
	})

	t.Run("head update empty", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).AnyTimes()
		fx.reqFactory.EXPECT().
			CreateFullSyncRequest(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullRequest, nil)

		res, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.NoError(t, err)
		require.Equal(t, fullRequest, res)
	})

	t.Run("head update empty equal heads", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      nil,
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h1"}).AnyTimes()

		res, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.NoError(t, err)
		require.Nil(t, res)
	})
}

func TestTreeSyncProtocol_FullSyncRequest(t *testing.T) {
	ctx := context.Background()
	fullResponse := &treechangeproto.TreeSyncMessage{
		Content: &treechangeproto.TreeSyncContentValue{
			Value: &treechangeproto.TreeSyncContentValue_FullSyncResponse{
				FullSyncResponse: &treechangeproto.TreeFullSyncResponse{},
			},
		},
	}

	t.Run("full sync request with change", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).AnyTimes()
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).AnyTimes().Return(false)
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)
		fx.reqFactory.EXPECT().
			CreateFullSyncResponse(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)

		res, err := fx.syncProtocol.FullSyncRequest(ctx, fx.senderId, fullSyncRequest)
		require.NoError(t, err)
		require.Equal(t, fullResponse, res)
	})

	t.Run("full sync request with change same heads", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"}).AnyTimes()
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)
		fx.reqFactory.EXPECT().
			CreateFullSyncResponse(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)

		res, err := fx.syncProtocol.FullSyncRequest(ctx, fx.senderId, fullSyncRequest)
		require.NoError(t, err)
		require.Equal(t, fullResponse, res)
	})

	t.Run("full sync request without changes", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.reqFactory.EXPECT().
			CreateFullSyncResponse(gomock.Eq(fx.objectTreeMock), gomock.Eq([]string{"h1"}), gomock.Eq([]string{"h1"})).
			Return(fullResponse, nil)

		res, err := fx.syncProtocol.FullSyncRequest(ctx, fx.senderId, fullSyncRequest)
		require.NoError(t, err)
		require.Equal(t, fullResponse, res)
	})

	t.Run("full sync request with change, raw changes error", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncRequest := &treechangeproto.TreeFullSyncRequest{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).AnyTimes()
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, fmt.Errorf("addRawChanges error"))

		_, err := fx.syncProtocol.FullSyncRequest(ctx, fx.senderId, fullSyncRequest)
		require.Error(t, err)
	})
}

func TestTreeSyncProtocol_FullSyncResponse(t *testing.T) {
	ctx := context.Background()

	t.Run("full sync response with change", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h2"}).AnyTimes()
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)

		err := fx.syncProtocol.FullSyncResponse(ctx, fx.senderId, fullSyncResponse)
		require.NoError(t, err)
	})

	t.Run("full sync response with same heads", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		fullSyncResponse := &treechangeproto.TreeFullSyncResponse{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}

		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().
			Heads().
			Return([]string{"h1"}).AnyTimes()
		fx.objectTreeMock.EXPECT().
			AddRawChanges(gomock.Any(), gomock.Eq(objecttree.RawChangesPayload{
				NewHeads:   []string{"h1"},
				RawChanges: []*treechangeproto.RawTreeChangeWithId{chWithId},
			})).
			Return(objecttree.AddResult{}, nil)

		err := fx.syncProtocol.FullSyncResponse(ctx, fx.senderId, fullSyncResponse)
		require.NoError(t, err)
	})
}
