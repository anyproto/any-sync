package syncacl

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/mock_list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/mock_syncacl"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"sync"
	"testing"
)

type testAclMock struct {
	*mock_list.MockAclList
	m sync.RWMutex
}

func newTestAclMock(mockAcl *mock_list.MockAclList) *testAclMock {
	return &testAclMock{
		MockAclList: mockAcl,
	}
}

func (t *testAclMock) Lock() {
	t.m.Lock()
}

func (t *testAclMock) RLock() {
	t.m.RLock()
}

func (t *testAclMock) Unlock() {
	t.m.Unlock()
}

func (t *testAclMock) RUnlock() {
	t.m.RUnlock()
}

func (t *testAclMock) TryLock() bool {
	return t.m.TryLock()
}

func (t *testAclMock) TryRLock() bool {
	return t.m.TryRLock()
}

type syncHandlerFixture struct {
	ctrl             *gomock.Controller
	syncClientMock   *mock_syncacl.MockSyncClient
	aclMock          *testAclMock
	syncProtocolMock *mock_syncacl.MockAclSyncProtocol
	spaceId          string
	senderId         string
	aclId            string

	syncHandler *syncAclHandler
}

func newSyncHandlerFixture(t *testing.T) *syncHandlerFixture {
	ctrl := gomock.NewController(t)
	aclMock := newTestAclMock(mock_list.NewMockAclList(ctrl))
	syncClientMock := mock_syncacl.NewMockSyncClient(ctrl)
	syncProtocolMock := mock_syncacl.NewMockAclSyncProtocol(ctrl)
	spaceId := "spaceId"

	syncHandler := &syncAclHandler{
		aclList:      aclMock,
		syncClient:   syncClientMock,
		syncProtocol: syncProtocolMock,
		syncStatus:   syncstatus.NewNoOpSyncStatus(),
		spaceId:      spaceId,
	}
	return &syncHandlerFixture{
		ctrl:             ctrl,
		syncClientMock:   syncClientMock,
		aclMock:          aclMock,
		syncProtocolMock: syncProtocolMock,
		spaceId:          spaceId,
		senderId:         "senderId",
		aclId:            "aclId",
		syncHandler:      syncHandler,
	}
}

func (fx *syncHandlerFixture) stop() {
	fx.ctrl.Finish()
}

func TestSyncAclHandler_HandleMessage(t *testing.T) {
	ctx := context.Background()
	t.Run("handle head update, request returned", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(logMessage, fx.spaceId, fx.aclId)

		syncReq := &consensusproto.LogSyncMessage{}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.syncProtocolMock.EXPECT().HeadUpdate(ctx, fx.senderId, gomock.Any()).Return(syncReq, nil)
		fx.syncClientMock.EXPECT().QueueRequest(fx.senderId, syncReq).Return(nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
	})
	t.Run("handle head update, no request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(logMessage, fx.spaceId, fx.aclId)

		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.syncProtocolMock.EXPECT().HeadUpdate(ctx, fx.senderId, gomock.Any()).Return(nil, nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
	})
	t.Run("handle head update, returned error", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(logMessage, fx.spaceId, fx.aclId)

		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		expectedErr := fmt.Errorf("some error")
		fx.syncProtocolMock.EXPECT().HeadUpdate(ctx, fx.senderId, gomock.Any()).Return(nil, expectedErr)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.Error(t, expectedErr, err)
	})
	t.Run("handle full sync request is forbidden", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(logMessage, fx.spaceId, fx.aclId)

		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.Error(t, ErrMessageIsRequest, err)
	})
	t.Run("handle full sync response, no error", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullResponse := &consensusproto.LogFullSyncResponse{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapFullResponse(fullResponse, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(logMessage, fx.spaceId, fx.aclId)

		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.syncProtocolMock.EXPECT().FullSyncResponse(ctx, fx.senderId, gomock.Any()).Return(nil)

		err := fx.syncHandler.HandleMessage(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
	})
}

func TestSyncAclHandler_HandleRequest(t *testing.T) {
	ctx := context.Background()
	t.Run("handle full sync request, no error", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(logMessage, fx.spaceId, fx.aclId)
		fullResp := &consensusproto.LogSyncMessage{
			Content: &consensusproto.LogSyncContentValue{
				Value: &consensusproto.LogSyncContentValue_FullSyncResponse{
					FullSyncResponse: &consensusproto.LogFullSyncResponse{
						Head: "returnedHead",
					},
				},
			},
		}

		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.syncProtocolMock.EXPECT().FullSyncRequest(ctx, fx.senderId, gomock.Any()).Return(fullResp, nil)
		res, err := fx.syncHandler.HandleRequest(ctx, fx.senderId, objectMsg)
		require.NoError(t, err)
		unmarshalled := &consensusproto.LogSyncMessage{}
		err = proto.Unmarshal(res.Payload, unmarshalled)
		if err != nil {
			return
		}
		require.Equal(t, "returnedHead", consensusproto.GetHead(unmarshalled))
	})
	t.Run("handle other message returns error", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		objectMsg, _ := spacesyncproto.MarshallSyncMessage(logMessage, fx.spaceId, fx.aclId)

		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		_, err := fx.syncHandler.HandleRequest(ctx, fx.senderId, objectMsg)
		require.Error(t, ErrMessageIsNotRequest, err)
	})
}
