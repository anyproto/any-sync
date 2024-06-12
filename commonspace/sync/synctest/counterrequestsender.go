package synctest

import (
	"context"
	"errors"
	"io"

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
		req, err := rq.Proto()
		if err != nil {
			return err
		}
		stream, err := cl.CounterStreamRequest(ctx, req.(*synctestproto.CounterRequest))
		if err != nil {
			return err
		}
		err = receive(stream)
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	})
}
