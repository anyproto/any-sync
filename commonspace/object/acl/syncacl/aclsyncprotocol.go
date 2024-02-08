//go:generate mockgen -destination mock_syncacl/mock_syncacl.go github.com/anyproto/any-sync/commonspace/object/acl/syncacl SyncAcl,SyncClient,RequestFactory,AclSyncProtocol
package syncacl

import (
	"context"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type AclSyncProtocol interface {
	HeadUpdate(ctx context.Context, senderId string, update *consensusproto.LogHeadUpdate) (request *consensusproto.LogSyncMessage, err error)
	FullSyncRequest(ctx context.Context, senderId string, request *consensusproto.LogFullSyncRequest) (response *consensusproto.LogSyncMessage, err error)
	FullSyncResponse(ctx context.Context, senderId string, response *consensusproto.LogFullSyncResponse) (err error)
}

type aclSyncProtocol struct {
	log        logger.CtxLogger
	spaceId    string
	aclList    list.AclList
	reqFactory RequestFactory
}

func (a *aclSyncProtocol) HeadUpdate(ctx context.Context, senderId string, update *consensusproto.LogHeadUpdate) (request *consensusproto.LogSyncMessage, err error) {
	isEmptyUpdate := len(update.Records) == 0
	log := a.log.With(
		zap.String("senderId", senderId),
		zap.String("update head", update.Head),
		zap.String("current head", a.aclList.Head().Id),
		zap.Int("len(update records)", len(update.Records)))
	log.DebugCtx(ctx, "received acl head update message")

	defer func() {
		if err != nil {
			log.ErrorCtx(ctx, "acl head update finished with error", zap.Error(err))
		} else if request != nil {
			cnt := request.Content.GetFullSyncRequest()
			log.DebugCtx(ctx, "returning acl full sync request", zap.String("request head", cnt.Head))
		} else {
			if !isEmptyUpdate {
				log.DebugCtx(ctx, "acl head update finished correctly")
			}
		}
	}()
	if isEmptyUpdate {
		headEquals := a.aclList.Head().Id == update.Head
		log.DebugCtx(ctx, "is empty acl head update", zap.Bool("headEquals", headEquals))
		if headEquals {
			return
		}
		return a.reqFactory.CreateFullSyncRequest(a.aclList, update.Head)
	}
	if a.aclList.HasHead(update.Head) {
		return
	}
	err = a.aclList.AddRawRecords(update.Records)
	if err == list.ErrIncorrectRecordSequence {
		return a.reqFactory.CreateFullSyncRequest(a.aclList, update.Head)
	}
	return
}

func (a *aclSyncProtocol) FullSyncRequest(ctx context.Context, senderId string, request *consensusproto.LogFullSyncRequest) (response *consensusproto.LogSyncMessage, err error) {
	log := a.log.With(
		zap.String("senderId", senderId),
		zap.String("request head", request.Head),
		zap.Int("len(request records)", len(request.Records)))
	log.DebugCtx(ctx, "received acl full sync request message")

	defer func() {
		if err != nil {
			log.ErrorCtx(ctx, "acl full sync request finished with error", zap.Error(err))
		} else if response != nil {
			cnt := response.Content.GetFullSyncResponse()
			log.DebugCtx(ctx, "acl full sync response sent", zap.String("response head", cnt.Head), zap.Int("len(response records)", len(cnt.Records)))
		}
	}()
	if !a.aclList.HasHead(request.Head) {
		if len(request.Records) > 0 {
			// in this case we can try to add some records
			err = a.aclList.AddRawRecords(request.Records)
			if err != nil {
				return
			}
		} else {
			// here it is impossible for us to do anything, we can't return records after head as defined in request, because we don't have it
			return nil, list.ErrIncorrectRecordSequence
		}
	}
	return a.reqFactory.CreateFullSyncResponse(a.aclList, request.Head)
}

func (a *aclSyncProtocol) FullSyncResponse(ctx context.Context, senderId string, response *consensusproto.LogFullSyncResponse) (err error) {
	log := a.log.With(
		zap.String("senderId", senderId),
		zap.String("response head", response.Head),
		zap.Int("len(response records)", len(response.Records)))
	log.DebugCtx(ctx, "received acl full sync response message")
	defer func() {
		if err != nil {
			log.ErrorCtx(ctx, "acl full sync response failed", zap.Error(err))
		} else {
			log.DebugCtx(ctx, "acl full sync response succeeded")
		}
	}()
	if a.aclList.HasHead(response.Head) {
		return
	}
	return a.aclList.AddRawRecords(response.Records)
}

func newAclSyncProtocol(spaceId string, aclList list.AclList, reqFactory RequestFactory) *aclSyncProtocol {
	return &aclSyncProtocol{
		log:        log.With(zap.String("spaceId", spaceId), zap.String("aclId", aclList.Id())),
		spaceId:    spaceId,
		aclList:    aclList,
		reqFactory: reqFactory,
	}
}
