package syncacl

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/mock_list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/mock_syncacl"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

type aclSyncProtocolFixture struct {
	log          logger.CtxLogger
	spaceId      string
	senderId     string
	aclId        string
	aclMock      *mock_list.MockAclList
	reqFactory   *mock_syncacl.MockRequestFactory
	ctrl         *gomock.Controller
	syncProtocol AclSyncProtocol
}

func newSyncProtocolFixture(t *testing.T) *aclSyncProtocolFixture {
	ctrl := gomock.NewController(t)
	aclList := mock_list.NewMockAclList(ctrl)
	spaceId := "spaceId"
	reqFactory := mock_syncacl.NewMockRequestFactory(ctrl)
	aclList.EXPECT().Id().Return("aclId")
	syncProtocol := newAclSyncProtocol(spaceId, aclList, reqFactory)
	return &aclSyncProtocolFixture{
		log:          log,
		spaceId:      spaceId,
		senderId:     "senderId",
		aclId:        "aclId",
		aclMock:      aclList,
		reqFactory:   reqFactory,
		ctrl:         ctrl,
		syncProtocol: syncProtocol,
	}
}

func (fx *aclSyncProtocolFixture) stop() {
	fx.ctrl.Finish()
}

func TestHeadUpdate(t *testing.T) {
	ctx := context.Background()
	fullRequest := &consensusproto.LogSyncMessage{
		Content: &consensusproto.LogSyncContentValue{
			Value: &consensusproto.LogSyncContentValue_FullSyncRequest{
				FullSyncRequest: &consensusproto.LogFullSyncRequest{},
			},
		},
	}
	t.Run("head update non empty all heads added", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		fx.aclMock.EXPECT().AddRawRecords(headUpdate.Records).Return(nil)
		req, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.Nil(t, req)
		require.NoError(t, err)
	})
	t.Run("head update results in full request", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		fx.aclMock.EXPECT().AddRawRecords(headUpdate.Records).Return(list.ErrIncorrectRecordSequence)
		fx.reqFactory.EXPECT().CreateFullSyncRequest(fx.aclMock, headUpdate.Head).Return(fullRequest, nil)
		req, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.Equal(t, fullRequest, req)
		require.NoError(t, err)
	})
	t.Run("head update old heads", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		headUpdate := &consensusproto.LogHeadUpdate{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(true)
		req, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.Nil(t, req)
		require.NoError(t, err)
	})
	t.Run("head update empty equals", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		headUpdate := &consensusproto.LogHeadUpdate{
			Head: "h1",
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "h1"})
		req, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.Nil(t, req)
		require.NoError(t, err)
	})
	t.Run("head update empty results in full request", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		headUpdate := &consensusproto.LogHeadUpdate{
			Head: "h1",
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().Head().Return(&list.AclRecord{Id: "h2"})
		fx.reqFactory.EXPECT().CreateFullSyncRequest(fx.aclMock, headUpdate.Head).Return(fullRequest, nil)
		req, err := fx.syncProtocol.HeadUpdate(ctx, fx.senderId, headUpdate)
		require.Equal(t, fullRequest, req)
		require.NoError(t, err)
	})
}

func TestFullSyncRequest(t *testing.T) {
	ctx := context.Background()
	fullResponse := &consensusproto.LogSyncMessage{
		Content: &consensusproto.LogSyncContentValue{
			Value: &consensusproto.LogSyncContentValue_FullSyncResponse{
				FullSyncResponse: &consensusproto.LogFullSyncResponse{},
			},
		},
	}
	t.Run("full sync request non empty all heads added", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		fx.aclMock.EXPECT().AddRawRecords(fullRequest.Records).Return(nil)
		fx.reqFactory.EXPECT().CreateFullSyncResponse(fx.aclMock, fullRequest.Head).Return(fullResponse, nil)
		resp, err := fx.syncProtocol.FullSyncRequest(ctx, fx.senderId, fullRequest)
		require.Equal(t, fullResponse, resp)
		require.NoError(t, err)
	})
	t.Run("full sync request non empty head exists", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(true)
		fx.reqFactory.EXPECT().CreateFullSyncResponse(fx.aclMock, fullRequest.Head).Return(fullResponse, nil)
		resp, err := fx.syncProtocol.FullSyncRequest(ctx, fx.senderId, fullRequest)
		require.Equal(t, fullResponse, resp)
		require.NoError(t, err)
	})
	t.Run("full sync request empty head not exists", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		fullRequest := &consensusproto.LogFullSyncRequest{
			Head: "h1",
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		resp, err := fx.syncProtocol.FullSyncRequest(ctx, fx.senderId, fullRequest)
		require.Nil(t, resp)
		require.Error(t, list.ErrIncorrectRecordSequence, err)
	})
}

func TestFullSyncResponse(t *testing.T) {
	ctx := context.Background()
	t.Run("full sync response no heads", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullResponse := &consensusproto.LogFullSyncResponse{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(false)
		fx.aclMock.EXPECT().AddRawRecords(fullResponse.Records).Return(nil)
		err := fx.syncProtocol.FullSyncResponse(ctx, fx.senderId, fullResponse)
		require.NoError(t, err)
	})
	t.Run("full sync response has heads", func(t *testing.T) {
		fx := newSyncProtocolFixture(t)
		defer fx.stop()
		chWithId := &consensusproto.RawRecordWithId{}
		fullResponse := &consensusproto.LogFullSyncResponse{
			Head:    "h1",
			Records: []*consensusproto.RawRecordWithId{chWithId},
		}
		fx.aclMock.EXPECT().Id().AnyTimes().Return(fx.aclId)
		fx.aclMock.EXPECT().HasHead("h1").Return(true)
		err := fx.syncProtocol.FullSyncResponse(ctx, fx.senderId, fullResponse)
		require.NoError(t, err)
	})
}
