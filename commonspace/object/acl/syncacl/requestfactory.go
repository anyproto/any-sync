package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type RequestFactory interface {
	CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (headUpdate *objectsync.HeadUpdate)
	CreateFullSyncRequest(peerId string, l list.AclList) *objectsync.Request
	CreateFullSyncResponse(l list.AclList, theirHead string) (resp *Response, err error)
}

type requestFactory struct {
	spaceId string
}

func NewRequestFactory(spaceId string) RequestFactory {
	return &requestFactory{spaceId: spaceId}
}

func (r *requestFactory) CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (headUpdate *objectsync.HeadUpdate) {
	return &objectsync.HeadUpdate{
		Meta: objectsync.ObjectMeta{
			ObjectId: l.Id(),
			SpaceId:  r.spaceId,
		},
		Update: InnerHeadUpdate{
			head:    l.Head().Id,
			records: added,
			root:    l.Root(),
		},
	}
}

func (r *requestFactory) CreateFullSyncRequest(peerId string, l list.AclList) *objectsync.Request {
	return NewRequest(peerId, l.Id(), r.spaceId, l.Head().Id, l.Root())
}

func (r *requestFactory) CreateFullSyncResponse(l list.AclList, theirHead string) (resp *Response, err error) {
	records, err := l.RecordsAfter(context.Background(), theirHead)
	if err != nil {
		return
	}
	return &Response{
		spaceId:  r.spaceId,
		objectId: l.Id(),
		head:     l.Head().Id,
		records:  records,
		root:     l.Root(),
	}, nil
}
