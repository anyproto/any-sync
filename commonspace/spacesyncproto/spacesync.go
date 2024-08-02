//go:generate mockgen -destination mock_spacesyncproto/mock_spacesyncproto.go github.com/anyproto/any-sync/commonspace/spacesyncproto DRPCSpaceSyncClient
package spacesyncproto

import (
	"github.com/anyproto/protobuf/proto"
	"storj.io/drpc"
)

type ObjectSyncStream = DRPCSpaceSync_ObjectSyncStreamStream

type ClientFactoryFunc func(cc drpc.Conn) DRPCSpaceSyncClient

func (c ClientFactoryFunc) Client(cc drpc.Conn) DRPCSpaceSyncClient {
	return c(cc)
}

type ClientFactory interface {
	Client(cc drpc.Conn) DRPCSpaceSyncClient
}

func MarshallSyncMessage(message proto.Marshaler, spaceId, objectId string) (objMsg *ObjectSyncMessage, err error) {
	payload, err := message.Marshal()
	if err != nil {
		return
	}
	objMsg = &ObjectSyncMessage{
		Payload:  payload,
		ObjectId: objectId,
		SpaceId:  spaceId,
	}
	return
}
