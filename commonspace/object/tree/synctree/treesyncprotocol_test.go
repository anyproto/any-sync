package synctree

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
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

func (fx *treeSyncProtocolFixture) finish() {
	fx.ctrl.Finish()
}

func TestTreeSyncProtocol_HeadUpdate(t *testing.T) {
	ctx := context.Background()
	//fullRequest := &treechangeproto.TreeSyncMessage{
	//	Content: &treechangeproto.TreeSyncContentValue{
	//		Value: &treechangeproto.TreeSyncContentValue_FullSyncRequest{
	//			FullSyncRequest: &treechangeproto.TreeFullSyncRequest{},
	//		},
	//	},
	//}
	t.Run("head update non empty all heads added", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		chWithId := &treechangeproto.RawTreeChangeWithId{}
		headUpdate := &treechangeproto.TreeHeadUpdate{
			Heads:        []string{"h1"},
			Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
			SnapshotPath: []string{"h1"},
		}
		fx.objectTreeMock.EXPECT().Id().AnyTimes().Return(fx.treeId)
		fx.objectTreeMock.EXPECT().Heads().Return([]string{"h2"}).Times(2)
		fx.objectTreeMock.EXPECT().HasChanges(gomock.Eq([]string{"h1"})).Return(false)
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
}
