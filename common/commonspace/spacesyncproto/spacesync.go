package spacesyncproto

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"

type SpaceStream = DRPCSpace_StreamStream

func WrapHeadUpdate(update *ObjectHeadUpdate, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_HeadUpdate{HeadUpdate: update},
		},
		RootChange: rootChange,
		TreeId:     treeId,
	}
}

func WrapFullRequest(request *ObjectFullSyncRequest, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_FullSyncRequest{FullSyncRequest: request},
		},
		RootChange: rootChange,
		TreeId:     treeId,
	}
}

func WrapFullResponse(response *ObjectFullSyncResponse, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_FullSyncResponse{FullSyncResponse: response},
		},
		RootChange: rootChange,
		TreeId:     treeId,
	}
}

func WrapError(err error, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_ErrorResponse{ErrorResponse: &ObjectErrorResponse{Error: err.Error()}},
		},
		RootChange: rootChange,
		TreeId:     treeId,
	}
}
