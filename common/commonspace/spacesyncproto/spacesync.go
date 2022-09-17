package spacesyncproto

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"

type SpaceStream = DRPCSpace_StreamStream

func WrapHeadUpdate(update *ObjectHeadUpdate, header *aclpb.TreeHeader, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_HeadUpdate{HeadUpdate: update},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}

func WrapFullRequest(request *ObjectFullSyncRequest, header *aclpb.TreeHeader, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_FullSyncRequest{FullSyncRequest: request},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}

func WrapFullResponse(response *ObjectFullSyncResponse, header *aclpb.TreeHeader, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_FullSyncResponse{FullSyncResponse: response},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}

func WrapError(err error, header *aclpb.TreeHeader, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_ErrorResponse{ErrorResponse: &ObjectErrorResponse{Error: err.Error()}},
		},
		TreeHeader: header,
		TreeId:     treeId,
	}
}
