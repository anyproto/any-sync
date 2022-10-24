//go:generate mockgen -destination mock_spacesyncproto/mock_spacesyncproto.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto DRPCSpaceClient
package spacesyncproto

import (
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
