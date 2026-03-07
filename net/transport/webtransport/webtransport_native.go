//go:build !js

package webtransport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	wt "github.com/quic-go/webtransport-go"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	netpeer "github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
)

var log = logger.NewNamed(CName)

func New() WebTransport {
	return new(wtTransport)
}

type wtTransport struct {
	secure      secureservice.SecureService
	localPeerId string
	accepter    transport.Accepter
	conf        Config

	server   *wt.Server
	udpConns []net.PacketConn

	listCtx       context.Context
	listCtxCancel context.CancelFunc
}

func (t *wtTransport) Init(a *app.App) (err error) {
	t.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	account := a.MustComponent(accountservice.CName).(accountservice.Service)
	t.localPeerId = account.Account().PeerId
	if cg, ok := a.Component("config").(configGetter); ok {
		t.conf = cg.GetWebTransport()
	}
	if t.conf.Path == "" {
		t.conf.Path = "/webtransport"
	}
	if t.conf.CloseTimeoutSec <= 0 {
		t.conf.CloseTimeoutSec = 5
	}
	if t.conf.DialTimeoutSec <= 0 {
		t.conf.DialTimeoutSec = 30
	}
	if t.conf.MaxStreams <= 0 {
		t.conf.MaxStreams = 128
	}
	return nil
}

func (t *wtTransport) Name() string {
	return CName
}

func (t *wtTransport) SetAccepter(accepter transport.Accepter) {
	t.accepter = accepter
}

func (t *wtTransport) Run(ctx context.Context) (err error) {
	if t.accepter == nil {
		return fmt.Errorf("can't run webtransport without accepter")
	}
	if len(t.conf.ListenAddrs) > 0 && (t.conf.CertFile == "" || t.conf.KeyFile == "") {
		return fmt.Errorf("CertFile and KeyFile are required when ListenAddrs is configured")
	}
	t.listCtx, t.listCtxCancel = context.WithCancel(context.Background())

	tlsConf := &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := tls.LoadX509KeyPair(t.conf.CertFile, t.conf.KeyFile)
			if err != nil {
				return nil, err
			}
			return &cert, nil
		},
		NextProtos: []string{http3.NextProtoH3},
	}

	quicConf := &quic.Config{
		EnableDatagrams:                  true,
		MaxIncomingStreams:               t.conf.MaxStreams,
		EnableStreamResetPartialDelivery: true,
	}

	h3Server := &http3.Server{
		TLSConfig:  tlsConf,
		QUICConfig: quicConf,
	}

	t.server = &wt.Server{
		H3:          h3Server,
		CheckOrigin: func(*http.Request) bool { return true },
	}

	mux := http.NewServeMux()
	mux.HandleFunc(t.conf.Path, t.handleUpgrade)
	h3Server.Handler = mux

	wt.ConfigureHTTP3Server(h3Server)

	for _, addr := range t.conf.ListenAddrs {
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return fmt.Errorf("resolve listen addr %q: %w", addr, err)
		}
		udpConn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			return fmt.Errorf("listen udp %q: %w", addr, err)
		}
		t.udpConns = append(t.udpConns, udpConn)

		log.Info("webtransport listener started",
			zap.String("addr", udpConn.LocalAddr().String()),
			zap.String("path", t.conf.Path),
		)
		go func() {
			if err := t.server.Serve(udpConn); err != nil {
				log.Debug("webtransport server stopped", zap.Error(err))
			}
		}()
	}
	return nil
}

func (t *wtTransport) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, CONNECT")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	sess, err := t.server.Upgrade(w, r)
	if err != nil {
		log.Info("webtransport upgrade failed", zap.Error(err))
		return
	}

	remotePeerId := r.URL.Query().Get("peerId")
	if remotePeerId == "" {
		log.Info("webtransport upgrade rejected: missing peerId query parameter", zap.String("remoteAddr", r.RemoteAddr))
		_ = sess.CloseWithError(2, "peerId query parameter required")
		return
	}
	remoteAddr := r.RemoteAddr
	go func() {
		if err := t.accept(sess, remoteAddr, remotePeerId); err != nil {
			log.Info("webtransport accept error", zap.Error(err), zap.String("remoteAddr", remoteAddr))
		}
	}()
}

func (t *wtTransport) accept(sess *wt.Session, remoteAddr, remotePeerId string) error {
	ctx, cancel := context.WithTimeout(t.listCtx, time.Duration(t.conf.DialTimeoutSec)*time.Second)
	defer cancel()

	stream, err := sess.AcceptStream(ctx)
	if err != nil {
		_ = sess.CloseWithError(1, "accept handshake stream failed")
		return fmt.Errorf("accept handshake stream: %w", err)
	}

	hsConn := wtNetConn{
		Stream:     stream,
		localAddr:  wtAddr{addr: "local"},
		remoteAddr: wtAddr{addr: remoteAddr},
	}

	cctx, err := t.secure.HandshakeInbound(ctx, hsConn, remotePeerId)
	if err != nil {
		_ = hsConn.Close()
		_ = sess.CloseWithError(3, "inbound handshake failed")
		return fmt.Errorf("handshake inbound: %w", err)
	}
	_ = hsConn.Close()

	mc := newConn(cctx, sess, remoteAddr,
		time.Duration(t.conf.WriteTimeoutSec)*time.Second,
		time.Duration(t.conf.CloseTimeoutSec)*time.Second,
	)
	return t.accepter.Accept(mc)
}

func (t *wtTransport) Dial(ctx context.Context, addr string) (transport.MultiConn, error) {
	expectedPeerId, _ := netpeer.CtxExpectedPeerId(ctx)
	if expectedPeerId == "" {
		return nil, fmt.Errorf("no expected peer id in context for WebTransport dial")
	}

	dialer := wt.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			MaxIncomingStreams:               t.conf.MaxStreams,
			EnableStreamResetPartialDelivery: true,
		},
	}

	dialURL := "https://" + addr + t.conf.Path + "?peerId=" + url.QueryEscape(t.localPeerId)
	_, sess, err := dialer.Dial(ctx, dialURL, nil)
	if err != nil {
		return nil, fmt.Errorf("webtransport dial: %w", err)
	}

	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		_ = sess.CloseWithError(1, "open handshake stream failed")
		return nil, fmt.Errorf("open handshake stream: %w", err)
	}

	hsConn := wtNetConn{
		Stream:     stream,
		localAddr:  wtAddr{addr: "local"},
		remoteAddr: wtAddr{addr: addr},
	}

	cctx, err := t.secure.HandshakeOutbound(ctx, hsConn, expectedPeerId)
	if err != nil {
		_ = hsConn.Close()
		_ = sess.CloseWithError(3, "outbound handshake failed")
		return nil, fmt.Errorf("handshake outbound: %w", err)
	}
	_ = hsConn.Close()

	return newConn(cctx, sess, addr,
		time.Duration(t.conf.WriteTimeoutSec)*time.Second,
		time.Duration(t.conf.CloseTimeoutSec)*time.Second,
	), nil
}

func (t *wtTransport) Close(ctx context.Context) error {
	if t.listCtxCancel != nil {
		t.listCtxCancel()
	}
	if t.server != nil {
		_ = t.server.Close()
	}
	for _, c := range t.udpConns {
		_ = c.Close()
	}
	return nil
}
