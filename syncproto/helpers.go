package syncproto

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

func WrapHeadUpdate(update *SyncHeadUpdate, header *aclpb.Header, treeId string) *Sync {
	return &Sync{
		Message: &SyncContentValue{
			Value: &SyncContentValueValueOfHeadUpdate{HeadUpdate: update},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}

func WrapFullRequest(request *SyncFullRequest, header *aclpb.Header, treeId string) *Sync {
	return &Sync{
		Message: &SyncContentValue{
			Value: &SyncContentValueValueOfFullSyncRequest{FullSyncRequest: request},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}

func WrapFullResponse(response *SyncFullResponse, header *aclpb.Header, treeId string) *Sync {
	return &Sync{
		Message: &SyncContentValue{
			Value: &SyncContentValueValueOfFullSyncResponse{FullSyncResponse: response},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}
