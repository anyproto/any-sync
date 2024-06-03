package synctest

import (
	"context"
	"fmt"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/sync/synctestproto"
)

type CounterRequestSender struct {
	peerProvider *PeerProvider
}

func (c *CounterRequestSender) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	peerId := rq.PeerId()
	fmt.Println("getting peer", peerId, rq.PeerId(), rq.ObjectId())
	pr, err := c.peerProvider.GetPeer(peerId)
	if err != nil {
		return err
	}
	fmt.Println("sending stream request with peer", pr.Id(), rq.PeerId(), rq.ObjectId())
	return pr.DoDrpc(ctx, func(conn drpc.Conn) error {
		fmt.Println("after connection", pr.Id(), rq.PeerId(), rq.ObjectId())
		cl := synctestproto.NewDRPCCounterSyncClient(conn)
		stream, err := cl.CounterStreamRequest(ctx, rq.Proto().(*synctestproto.CounterRequest))
		if err != nil {
			return err
		}
		return receive(stream)
	})
}
