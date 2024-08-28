package syncacl

import (
	"sync"
	"testing"

	"github.com/anyproto/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/mock_list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/mock_syncacl"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/response"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/syncstatus/mock_syncstatus"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/peer"
)

type testQueueUpdater struct {
}

func (t testQueueUpdater) UpdateQueueSize(size uint64, msgType int, add bool) {
}

func TestSyncAclHandler_HandleHeadUpdate(t *testing.T) {
	ctx = peer.CtxWithPeerId(ctx, "peerId")
	t.Run("handle head update, can't add records, request returned", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		marshaled, err := logMessage.Marshal()
		require.NoError(t, err)
		objectHeadUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		retReq := &objectmessages.Request{
			Bytes: []byte("bytes"),
		}
		fx.statusUpdater.EXPECT().HeadsReceive("peerId", "objectId", []string{"h1"})
		fx.aclMock.EXPECT().AddRawRecords([]*consensusproto.RawRecordWithId{chWithId}).Return(list.ErrIncorrectRecordSequence)
		fx.syncClientMock.EXPECT().CreateFullSyncRequest("peerId", fx.aclMock).Return(retReq)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.statusUpdater, objectHeadUpdate)
		require.NoError(t, err)
		require.Equal(t, retReq, req)
	})
	t.Run("handle head update, records added, no request returned", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		marshaled, err := logMessage.Marshal()
		require.NoError(t, err)
		objectHeadUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		fx.statusUpdater.EXPECT().HeadsReceive("peerId", "objectId", []string{"h1"})
		fx.aclMock.EXPECT().AddRawRecords([]*consensusproto.RawRecordWithId{chWithId}).Return(nil)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.statusUpdater, objectHeadUpdate)
		require.NoError(t, err)
		require.Nil(t, req)
	})
	t.Run("handle head update, no records, request returned", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head: "h1",
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		marshaled, err := logMessage.Marshal()
		require.NoError(t, err)
		objectHeadUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		retReq := &objectmessages.Request{
			Bytes: []byte("bytes"),
		}
		fx.statusUpdater.EXPECT().HeadsReceive("peerId", "objectId", []string{"h1"})
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		fx.syncClientMock.EXPECT().CreateFullSyncRequest("peerId", fx.aclMock).Return(retReq)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.statusUpdater, objectHeadUpdate)
		require.NoError(t, err)
		require.Equal(t, retReq, req)
	})
	t.Run("handle head update, no records, same heads", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head: "h1",
		}
		logMessage := consensusproto.WrapHeadUpdate(headUpdate, chWithId)
		marshaled, err := logMessage.Marshal()
		require.NoError(t, err)
		objectHeadUpdate := &objectmessages.HeadUpdate{
			Bytes: marshaled,
			Meta: objectmessages.ObjectMeta{
				PeerId:   "peerId",
				ObjectId: "objectId",
				SpaceId:  "spaceId",
			},
		}
		fx.statusUpdater.EXPECT().HeadsReceive("peerId", "objectId", []string{"h1"})
		fx.aclMock.EXPECT().HasHead("h1").Return(true)
		req, err := fx.syncHandler.HandleHeadUpdate(ctx, fx.statusUpdater, objectHeadUpdate)
		require.NoError(t, err)
		require.Nil(t, req)
	})
}

func TestSyncAclHandler_HandleStreamRequest(t *testing.T) {
	t.Run("handle full sync request, no head exists, return request", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapFullRequest(fullRequest, chWithId)
		marshaled, err := logMessage.Marshal()
		require.NoError(t, err)
		returnReq := &objectmessages.Request{Bytes: []byte("bytes")}
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		fx.syncClientMock.EXPECT().CreateFullSyncRequest("peerId", fx.aclMock).Return(returnReq)
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testQueueUpdater{}, func(resp proto.Message) error {
			return nil
		})
		require.Equal(t, ErrUnknownHead, err)
		require.Equal(t, returnReq, req)
	})
	t.Run("handle full sync request, head exists, return response", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapFullRequest(fullRequest, chWithId)
		marshaled, err := logMessage.Marshal()
		require.NoError(t, err)
		returnResp := &response.Response{Head: "h2"}
		request := objectmessages.NewByteRequest("peerId", "spaceId", "objectId", marshaled)
		fx.aclMock.EXPECT().HasHead("h1").Return(true)
		fx.syncClientMock.EXPECT().CreateFullSyncResponse(fx.aclMock, "h1").Return(returnResp, nil)
		sendCalled := false
		req, err := fx.syncHandler.HandleStreamRequest(ctx, request, testQueueUpdater{}, func(resp proto.Message) error {
			require.NotNil(t, resp)
			sendCalled = true
			return nil
		})
		require.NoError(t, err)
		require.Nil(t, req)
		require.True(t, sendCalled)
	})
}

func TestSyncAclHandler_HandleDeprecatedRequest(t *testing.T) {
	t.Run("handle deprecated request, records, return empty response", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		logMessage := consensusproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, err := spacesyncproto.MarshallSyncMessage(logMessage, "spaceId", "objectId")
		require.NoError(t, err)
		fx.aclMock.EXPECT().Root().Return(chWithId)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "h2"})
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		fx.aclMock.EXPECT().AddRawRecords([]*consensusproto.RawRecordWithId{chWithId}).Return(nil)
		resp, err := fx.syncHandler.HandleDeprecatedRequest(ctx, objectMsg)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, "spaceId", resp.SpaceId)
		require.Equal(t, "objectId", resp.ObjectId)
	})
	t.Run("handle deprecated request, no records, return empty response", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head: "h1",
		}
		logMessage := consensusproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, err := spacesyncproto.MarshallSyncMessage(logMessage, "spaceId", "objectId")
		require.NoError(t, err)
		fx.aclMock.EXPECT().Root().Return(chWithId)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "h2"})
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		resp, err := fx.syncHandler.HandleDeprecatedRequest(ctx, objectMsg)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, "spaceId", resp.SpaceId)
		require.Equal(t, "objectId", resp.ObjectId)
	})
	t.Run("handle deprecated request, has head, return records after", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head: "h1",
		}
		logMessage := consensusproto.WrapFullRequest(fullRequest, chWithId)
		objectMsg, err := spacesyncproto.MarshallSyncMessage(logMessage, "spaceId", "objectId")
		require.NoError(t, err)
		fx.aclMock.EXPECT().Root().Return(chWithId)
		fx.aclMock.EXPECT().Head().Times(2).Return(&list.AclRecord{Id: "h2"})
		fx.aclMock.EXPECT().HasHead("h1").Return(true)
		fx.aclMock.EXPECT().RecordsAfter(ctx, "h1").Return([]*consensusproto.RawRecordWithId{chWithId}, nil)
		resp, err := fx.syncHandler.HandleDeprecatedRequest(ctx, objectMsg)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, "spaceId", resp.SpaceId)
		require.Equal(t, "objectId", resp.ObjectId)
	})
}

func TestSyncAclHandler_HandleResponse(t *testing.T) {
	t.Run("handle response, no changes, return nil", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		resp := &response.Response{}
		err := fx.syncHandler.HandleResponse(ctx, "peerId", "objectId", resp)
		require.NoError(t, err)
	})
	t.Run("handle response, changes, add records", func(t *testing.T) {
		fx := newSyncHandlerFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		resp := &response.Response{
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().AddRawRecords([]*consensusproto.RawRecordWithId{chWithId}).Return(nil)
		err := fx.syncHandler.HandleResponse(ctx, "peerId", "objectId", resp)
		require.NoError(t, err)
	})
}

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
	ctrl           *gomock.Controller
	syncClientMock *mock_syncacl.MockSyncClient
	statusUpdater  *mock_syncstatus.MockStatusUpdater
	aclMock        *testAclMock

	syncHandler *syncAclHandler
}

func newSyncHandlerFixture(t *testing.T) *syncHandlerFixture {
	ctrl := gomock.NewController(t)
	aclMock := newTestAclMock(mock_list.NewMockAclList(ctrl))
	syncClientMock := mock_syncacl.NewMockSyncClient(ctrl)
	statusUpdater := mock_syncstatus.NewMockStatusUpdater(ctrl)

	syncHandler := &syncAclHandler{
		aclList:    aclMock,
		syncClient: syncClientMock,
		spaceId:    "spaceId",
	}
	return &syncHandlerFixture{
		ctrl:           ctrl,
		syncClientMock: syncClientMock,
		aclMock:        aclMock,
		statusUpdater:  statusUpdater,
		syncHandler:    syncHandler,
	}
}

func (fx *syncHandlerFixture) stop() {
	fx.ctrl.Finish()
}
