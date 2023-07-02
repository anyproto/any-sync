package syncacl

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type RequestFactory interface {
	CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (msg *consensusproto.LogSyncMessage)
	CreateFullSyncRequest(theirHead string) (req *consensusproto.LogSyncMessage, err error)
	CreateFullSyncResponse(l list.AclList, theirHead string) (*consensusproto.LogSyncMessage, error)
}

func NewRequestFactory() RequestFactory {
	return &requestFactory{}
}

type requestFactory struct{}

func (r *requestFactory) CreateHeadUpdate(l list.AclList, added []*consensusproto.RawRecordWithId) (msg *consensusproto.LogSyncMessage) {
	return
}

func (r *requestFactory) CreateFullSyncRequest(theirHead string) (req *consensusproto.LogSyncMessage, err error) {
	return
}

func (r *requestFactory) CreateFullSyncResponse(l list.AclList, theirHead string) (*consensusproto.LogSyncMessage, error) {
	return nil, nil
}
