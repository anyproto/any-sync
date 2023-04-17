package dialer

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	net2 "github.com/anytypeio/any-sync/net"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/secureservice"
	"github.com/anytypeio/any-sync/net/timeoutconn"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/libp2p/go-libp2p/core/sec"
	"go.uber.org/zap"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcwire"
	"sync"
	"time"
)

const CName = "common.net.dialer"

var ErrArrdsNotFound = errors.New("addrs for peer not found")

var log = logger.NewNamed(CName)

func New() Dialer {
	return &dialer{}
}

type Dialer interface {
	Dial(ctx context.Context, peerId string) (peer peer.Peer, err error)
	UpdateAddrs(addrs map[string][]string)
	SetPeerAddrs(peerId string, addrs []string)
	app.Component
}

type dialer struct {
	transport secureservice.SecureService
	config    net2.Config
	nodeConf  nodeconf.NodeConf
	peerAddrs map[string][]string

	mu sync.RWMutex
}

func (d *dialer) Init(a *app.App) (err error) {
	d.transport = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	d.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	d.config = a.MustComponent("config").(net2.ConfigGetter).GetNet()
	return
}

func (d *dialer) Name() (name string) {
	return CName
}

func (d *dialer) UpdateAddrs(addrs map[string][]string) {
	d.mu.Lock()
	d.peerAddrs = addrs
	d.mu.Unlock()
}

func (d *dialer) SetPeerAddrs(peerId string, addrs []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.peerAddrs == nil {
		return
	}
	d.peerAddrs[peerId] = addrs
}

func (d *dialer) getPeerAddrs(peerId string) ([]string, error) {
	if addrs, ok := d.nodeConf.PeerAddresses(peerId); ok {
		return addrs, nil
	}
	addrs, ok := d.peerAddrs[peerId]
	if !ok || len(addrs) == 0 {
		return nil, ErrArrdsNotFound
	}
	return addrs, nil
}

func (d *dialer) Dial(ctx context.Context, peerId string) (p peer.Peer, err error) {
	var ctxCancel context.CancelFunc
	ctx, ctxCancel = context.WithTimeout(ctx, time.Second*10)
	defer ctxCancel()
	d.mu.RLock()
	defer d.mu.RUnlock()

	addrs, err := d.getPeerAddrs(peerId)
	if err != nil {
		return
	}

	var (
		conn drpc.Conn
		sc   sec.SecureConn
	)
	log.InfoCtx(ctx, "dial", zap.String("peerId", peerId), zap.Strings("addrs", addrs))
	for _, addr := range addrs {
		conn, sc, err = d.handshake(ctx, addr)
		if err != nil {
			log.InfoCtx(ctx, "can't connect to host", zap.String("addr", addr), zap.Error(err))
		} else {
			break
		}
	}
	if err != nil {
		return
	}
	return peer.NewPeer(sc, conn), nil
}

func (d *dialer) handshake(ctx context.Context, addr string) (conn drpc.Conn, sc sec.SecureConn, err error) {
	st := time.Now()
	// TODO: move dial timeout to config
	tcpConn, err := net.DialTimeout("tcp", addr, time.Second*3)
	if err != nil {
		return nil, nil, fmt.Errorf("dialTimeout error: %v; since start: %v", err, time.Since(st))
	}

	timeoutConn := timeoutconn.NewConn(tcpConn, time.Millisecond*time.Duration(d.config.Stream.TimeoutMilliseconds))
	sc, err = d.transport.SecureOutbound(ctx, timeoutConn)
	if err != nil {
		return nil, nil, fmt.Errorf("tls handshaeke error: %v; since start: %v", err, time.Since(st))
	}
	log.Info("connected with remote host", zap.String("serverPeer", sc.RemotePeer().String()), zap.String("addr", addr))
	conn = drpcconn.NewWithOptions(sc, drpcconn.Options{Manager: drpcmanager.Options{
		Reader: drpcwire.ReaderOptions{MaximumBufferSize: d.config.Stream.MaxMsgSizeMb * (1 << 20)},
	}})
	return conn, sc, err
}
