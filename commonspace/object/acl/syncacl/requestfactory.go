package syncacl

import (
	"context"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type RequestFactory interface {
	CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (headUpdate *objectmessages.HeadUpdate, err error)
	CreateFullSyncRequest(peerId string, l list.AclList) *objectmessages.Request
	CreateFullSyncResponse(l list.AclList, theirHead string) (resp *Response, err error)
}

type requestFactory struct {
	spaceId string
}

func NewRequestFactory(spaceId string) RequestFactory {
	return &requestFactory{spaceId: spaceId}
}

func (r *requestFactory) CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (headUpdate *objectmessages.HeadUpdate, err error) {
	headUpdate = &objectmessages.HeadUpdate{
		Meta: objectmessages.ObjectMeta{
			ObjectId: l.Id(),
			SpaceId:  r.spaceId,
		},
		Update: &InnerHeadUpdate{
			head:    l.Head().Id,
			records: added,
			root:    l.Root(),
		},
	}
	err = headUpdate.Update.Prepare()
	return
}

func (r *requestFactory) CreateFullSyncRequest(peerId string, l list.AclList) *objectmessages.Request {
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
