//go:generate mockgen -destination mock_spacesyncproto/mock_spacesyncproto.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto DRPCSpaceSyncClient
package spacesyncproto

import (
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
