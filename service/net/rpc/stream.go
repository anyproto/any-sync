package rpc

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/libp2p/go-libp2p-core/sec"
	"storj.io/drpc"
	"sync/atomic"
	"time"
)

func PeerFromStream(sc sec.SecureConn, stream drpc.Stream, incoming bool) peer.Peer {
	dp := &drpcPeer{
		sc:     sc,
		Stream: stream,
	}
	dp.info.Id = sc.RemotePeer().String()
	if incoming {
		dp.info.Dir = peer.DirInbound
	} else {
		dp.info.Dir = peer.DirOutbound
	}
	return dp
}

type drpcPeer struct {
	sc   sec.SecureConn
	info peer.Info
	drpc.Stream
}

func (d *drpcPeer) Id() string {
	return d.info.Id
}

func (d *drpcPeer) Info() peer.Info {
	return d.info
}

func (d *drpcPeer) Recv() (msg *syncproto.Message, err error) {
	msg = &syncproto.Message{}
	if err = d.Stream.MsgRecv(msg, Encoding); err != nil {
		return
	}
	atomic.StoreInt64(&d.info.LastActiveUnix, time.Now().Unix())
	return
}

func (d *drpcPeer) Send(msg *syncproto.Message) (err error) {
	if err = d.Stream.MsgSend(msg, Encoding); err != nil {
		return
	}
	atomic.StoreInt64(&d.info.LastActiveUnix, time.Now().Unix())
	return
}
