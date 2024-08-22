package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type RequestFactory interface {
	CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (msg *consensusproto.LogSyncMessage)
	CreateEmptyFullSyncRequest(l list.AclList) (req *consensusproto.LogSyncMessage)
	CreateFullSyncRequest(l list.AclList, theirHead string) (req *consensusproto.LogSyncMessage, err error)
	CreateFullSyncResponse(l list.AclList, theirHead string) (*consensusproto.LogSyncMessage, error)
}

type requestFactory struct{}

func NewRequestFactory() RequestFactory {
	return &requestFactory{}
}

func (r *requestFactory) CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (msg *consensusproto.LogSyncMessage) {
	return consensusproto.WrapHeadUpdate(&consensusproto.LogHeadUpdate{
		Head:    l.Head().Id,
		Records: added,
	}, l.Root())
}

func (r *requestFactory) CreateEmptyFullSyncRequest(l list.AclList) (req *consensusproto.LogSyncMessage) {
	// this is only sent to newer versions of the protocol
	return consensusproto.WrapFullRequest(&consensusproto.LogFullSyncRequest{
		Head: l.Head().Id,
	}, l.Root())
}

func (r *requestFactory) CreateFullSyncRequest(l list.AclList, theirHead string) (req *consensusproto.LogSyncMessage, err error) {
	if !l.HasHead(theirHead) {
		return consensusproto.WrapFullRequest(&consensusproto.LogFullSyncRequest{
			Head: l.Head().Id,
		}, l.Root()), nil
	}
	records, err := l.RecordsAfter(context.Background(), theirHead)
	if err != nil {
		return
	}
	return consensusproto.WrapFullRequest(&consensusproto.LogFullSyncRequest{
		Head:    l.Head().Id,
		Records: records,
	}, l.Root()), nil
}

func (r *requestFactory) CreateFullSyncResponse(l list.AclList, theirHead string) (resp *consensusproto.LogSyncMessage, err error) {
	records, err := l.RecordsAfter(context.Background(), theirHead)
	if err != nil {
		return
	}
	return consensusproto.WrapFullResponse(&consensusproto.LogFullSyncResponse{
		Head:    l.Head().Id,
		Records: records,
	}, l.Root()), nil
}
