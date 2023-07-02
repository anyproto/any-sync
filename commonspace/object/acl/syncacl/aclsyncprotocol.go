package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"go.uber.org/zap"
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
	return
}

func (a *aclSyncProtocol) FullSyncRequest(ctx context.Context, senderId string, request *consensusproto.LogFullSyncRequest) (response *consensusproto.LogSyncMessage, err error) {
	return
}

func (a *aclSyncProtocol) FullSyncResponse(ctx context.Context, senderId string, response *consensusproto.LogFullSyncResponse) (err error) {
	return
}

func newAclSyncProtocol(spaceId string, aclList list.AclList, reqFactory RequestFactory) *aclSyncProtocol {
	return &aclSyncProtocol{
		log:        log.With(zap.String("spaceId", spaceId), zap.String("aclId", aclList.Id())),
		spaceId:    spaceId,
		aclList:    aclList,
		reqFactory: reqFactory,
	}
}
