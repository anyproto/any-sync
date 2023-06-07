package quic

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/quic-go/quic-go"
	"go.uber.org/zap"
	"net"
	"time"
)

const CName = "net.transport.quic"

var log = logger.NewNamed(CName)

func New() Quic {
	return new(quicTransport)
}

type Quic interface {
	transport.Transport
	app.ComponentRunnable
}

type quicTransport struct {
	secure        secureservice.SecureService
	accepter      transport.Accepter
	conf          Config
	quicConf      *quic.Config
	listeners     []*quic.Listener
	listCtx       context.Context
	listCtxCancel context.CancelFunc
}

func (q *quicTransport) Init(a *app.App) (err error) {
	q.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	q.conf = a.MustComponent("config").(configGetter).GetQuic()
	q.quicConf = &quic.Config{
		HandshakeIdleTimeout: time.Duration(q.conf.DialTimeoutSec) * time.Second,
		MaxIncomingStreams:   q.conf.MaxStreams,
	}
	return
}

func (q *quicTransport) Name() (name string) {
	return CName
}

func (q *quicTransport) SetAccepter(accepter transport.Accepter) {
	q.accepter = accepter
}

func (q *quicTransport) Run(ctx context.Context) (err error) {
	if q.accepter == nil {
		return fmt.Errorf("can't run service without accepter")
	}
	tlConf, err := q.secure.ServerTlsConfig()
	if err != nil {
		return
	}
	for _, listAddr := range q.conf.ListenAddrs {
		list, err := quic.ListenAddr(listAddr, tlConf, q.quicConf)
		if err != nil {
			return err
		}
		q.listeners = append(q.listeners, list)
	}
	q.listCtx, q.listCtxCancel = context.WithCancel(context.Background())
	for _, list := range q.listeners {
		go q.acceptLoop(q.listCtx, list)
	}
	return
}

func (q *quicTransport) Dial(ctx context.Context, addr string) (mc transport.MultiConn, err error) {
	//TODO implement me
	panic("implement me")
}

func (q *quicTransport) acceptLoop(ctx context.Context, list *quic.Listener) {
	l := log.With(zap.String("localAddr", list.Addr().String()))
	l.Info("quic listener started")
	defer func() {
		l.Debug("quic listener stopped")
	}()
	for {
		conn, err := list.Accept(ctx)
		if err != nil {
			if err != net.ErrClosed {
				l.Error("listener closed with error", zap.Error(err))
			}
			return
		}
		go func() {
			if err := q.accept(conn); err != nil {
				l.Info("accept error", zap.Error(err))
			}
		}()
	}
}

func (q *quicTransport) accept(conn quic.Connection) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(q.conf.DialTimeoutSec))
	defer cancel()
	remotePubKey, err := libp2ptls.PubKeyFromCertChain(conn.ConnectionState().TLS.PeerCertificates)
	if err != nil {
		return err
	}
	remotePeerId, err := peer.IDFromPublicKey(remotePubKey)
	if err != nil {
		return err
	}

	// wait new stream for any handshake
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = stream.Close()
	}()
	cctx, err := q.secure.HandshakeInbound(ctx, stream, remotePeerId.String())
	if err != nil {
		return
	}
	mc := newConn(cctx, conn)
	return q.accepter.Accept(mc)
}

func (q *quicTransport) Close(ctx context.Context) (err error) {
	if q.listCtx != nil {
		q.listCtxCancel()
	}
	for _, lis := range q.listeners {
		_ = lis.Close()
	}
	return
}
