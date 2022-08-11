package syncproto

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"

func WrapHeadUpdate(update *SyncHeadUpdate, header *treepb.TreeHeader, treeId string) *Sync {
	return &Sync{
		Message: &SyncContentValue{
			Value: &SyncContentValueValueOfHeadUpdate{HeadUpdate: update},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}

func WrapFullRequest(request *SyncFullRequest, header *treepb.TreeHeader, treeId string) *Sync {
	return &Sync{
		Message: &SyncContentValue{
			Value: &SyncContentValueValueOfFullSyncRequest{FullSyncRequest: request},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}

func WrapFullResponse(response *SyncFullResponse, header *treepb.TreeHeader, treeId string) *Sync {
	return &Sync{
		Message: &SyncContentValue{
			Value: &SyncContentValueValueOfFullSyncResponse{FullSyncResponse: response},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}
