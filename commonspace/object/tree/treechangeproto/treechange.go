package treechangeproto

import "github.com/anytypeio/any-sync/net/rpc/rpcerr"

func WrapHeadUpdate(update *TreeHeadUpdate, rootChange *RawTreeChangeWithId) *TreeSyncMessage {
	return &TreeSyncMessage{
		Content: &TreeSyncContentValue{
			Value: &TreeSyncContentValue_HeadUpdate{HeadUpdate: update},
		},
		RootChange: rootChange,
	}
}

func WrapFullRequest(request *TreeFullSyncRequest, rootChange *RawTreeChangeWithId) *TreeSyncMessage {
	return &TreeSyncMessage{
		Content: &TreeSyncContentValue{
			Value: &TreeSyncContentValue_FullSyncRequest{FullSyncRequest: request},
		},
		RootChange: rootChange,
	}
}

func WrapFullResponse(response *TreeFullSyncResponse, rootChange *RawTreeChangeWithId) *TreeSyncMessage {
	return &TreeSyncMessage{
		Content: &TreeSyncContentValue{
			Value: &TreeSyncContentValue_FullSyncResponse{FullSyncResponse: response},
		},
		RootChange: rootChange,
	}
}

func WrapError(err error, rootChange *RawTreeChangeWithId) *TreeSyncMessage {
	return &TreeSyncMessage{
		Content: &TreeSyncContentValue{
			Value: &TreeSyncContentValue_ErrorResponse{ErrorResponse: &TreeErrorResponse{ErrCode: rpcerr.Code(err)}},
		},
		RootChange: rootChange,
	}
}

func GetHeads(msg *TreeSyncMessage) (heads []string) {
	content := msg.GetContent()
	switch {
	case content.GetHeadUpdate() != nil:
		return content.GetHeadUpdate().Heads
	case content.GetFullSyncRequest() != nil:
		return content.GetFullSyncRequest().Heads
	case content.GetFullSyncResponse() != nil:
		return content.GetFullSyncResponse().Heads
	default:
		return nil
	}
}
