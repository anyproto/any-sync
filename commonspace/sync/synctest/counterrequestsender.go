package synctest

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

type CounterRequestSender struct {
	peerProvider *PeerProvider
}

func (c *CounterRequestSender) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	peerId := rq.PeerId()
	pr, err := c.peerProvider.GetPeer(peerId)
	if err != nil {
		return err
	}
	return pr.DoDrpc(ctx, func(conn drpc.Conn) error {
		cl := synctestproto.NewDRPCCounterSyncClient(conn)
		stream, err := cl.CounterStreamRequest(ctx, rq.Proto().(*synctestproto.CounterRequest))
		if err != nil {
			return err
		}
		return receive(stream)
	})
}
