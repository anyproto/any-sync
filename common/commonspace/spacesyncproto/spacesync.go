//go:generate mockgen -destination mock_spacesyncproto/mock_spacesyncproto.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto DRPCSpaceClient
package spacesyncproto

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"storj.io/drpc"
)

type SpaceStream = DRPCSpace_StreamStream

type ClientFactoryFunc func(cc drpc.Conn) DRPCSpaceClient

func (c ClientFactoryFunc) Client(cc drpc.Conn) DRPCSpaceClient {
	return c(cc)
}

type ClientFactory interface {
	Client(cc drpc.Conn) DRPCSpaceClient
}

func WrapHeadUpdate(update *ObjectHeadUpdate, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_HeadUpdate{HeadUpdate: update},
		},
		RootChange: rootChange,
		TreeId:     treeId,
		TrackingId: trackingId,
	}
}

func WrapFullRequest(request *ObjectFullSyncRequest, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_FullSyncRequest{FullSyncRequest: request},
		},
		RootChange: rootChange,
		TreeId:     treeId,
		TrackingId: trackingId,
	}
}

func WrapFullResponse(response *ObjectFullSyncResponse, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_FullSyncResponse{FullSyncResponse: response},
		},
		RootChange: rootChange,
		TreeId:     treeId,
		TrackingId: trackingId,
	}
}

func WrapError(err error, rootChange *treechangeproto.RawTreeChangeWithId, treeId, trackingId string) *ObjectSyncMessage {
	return &ObjectSyncMessage{
		Content: &ObjectSyncContentValue{
			Value: &ObjectSyncContentValue_ErrorResponse{ErrorResponse: &ObjectErrorResponse{Error: err.Error()}},
		},
		RootChange: rootChange,
		TreeId:     treeId,
		TrackingId: trackingId,
	}
}

func MessageDescription(msg *ObjectSyncMessage) (res string) {
	content := msg.GetContent()
	switch {
	case content.GetHeadUpdate() != nil:
		res = fmt.Sprintf("head update/%v", content.GetHeadUpdate().Heads)
	case content.GetFullSyncRequest() != nil:
		res = fmt.Sprintf("fullsync request/%v", content.GetFullSyncRequest().Heads)
	case content.GetFullSyncResponse() != nil:
		res = fmt.Sprintf("fullsync response/%v", content.GetFullSyncResponse().Heads)
	case content.GetErrorResponse() != nil:
		res = fmt.Sprintf("error response/%v", content.GetErrorResponse().Error)
	}
	res = fmt.Sprintf("%s/tracking=[%s]", res, msg.TrackingId)
	return res
}
