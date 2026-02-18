//go:build !js

package webrtc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	netpeer "github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
)

var log = logger.NewNamed(CName)

func New() WebRTC {
	return new(webrtcTransport)
}

type webrtcTransport struct {
	secure      secureservice.SecureService
	localPeerId string
	accepter    transport.Accepter
	conf        Config

	signalServer   *http.Server
	signalListener net.Listener

	listCtx       context.Context
	listCtxCancel context.CancelFunc
	mu            sync.Mutex
}

func (t *webrtcTransport) Init(a *app.App) (err error) {
	t.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	account := a.MustComponent(accountservice.CName).(accountservice.Service)
	t.localPeerId = account.Account().PeerId
	if cg, ok := a.Component("config").(configGetter); ok {
		t.conf = cg.GetWebRTC()
	}
	if t.conf.DialTimeoutSec <= 0 {
		t.conf.DialTimeoutSec = 30
	}
	if t.conf.CloseTimeoutSec <= 0 {
		t.conf.CloseTimeoutSec = 5
	}
	return nil
}

func (t *webrtcTransport) Name() string {
	return CName
}

func (t *webrtcTransport) SetAccepter(accepter transport.Accepter) {
	t.accepter = accepter
}

func (t *webrtcTransport) Run(ctx context.Context) (err error) {
	if t.accepter == nil {
		return fmt.Errorf("can't run webrtc transport without accepter")
	}
	t.listCtx, t.listCtxCancel = context.WithCancel(context.Background())

	for _, addr := range t.conf.ListenAddrs {
		if err = t.listenAddr(addr); err != nil {
			return err
		}
	}
	return nil
}

// newPeerConnection creates a new PeerConnection with detached DataChannels.
func (t *webrtcTransport) newPeerConnection() (*webrtc.PeerConnection, error) {
	se := webrtc.SettingEngine{}
	se.DetachDataChannels()
	api := webrtc.NewAPI(webrtc.WithSettingEngine(se))
	return api.NewPeerConnection(webrtc.Configuration{})
}

func (t *webrtcTransport) listenAddr(addr string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	signalPort := t.conf.SignalPort
	signalAddr := addr
	if signalPort > 0 {
		host, _, _ := net.SplitHostPort(addr)
		signalAddr = fmt.Sprintf("%s:%d", host, signalPort)
	}

	mux := http.NewServeMux()
	mux.Handle(signalPath, &signalHandler{
		handleOffer: t.handleOffer,
	})
	t.signalServer = &http.Server{
		Handler: mux,
	}

	signalListener, err := net.Listen("tcp", signalAddr)
	if err != nil {
		return fmt.Errorf("listen signal: %w", err)
	}
	t.signalListener = signalListener

	log.Info("webrtc transport started",
		zap.Int("signalPort", signalListener.Addr().(*net.TCPAddr).Port),
	)

	go func() {
		if err := t.signalServer.Serve(signalListener); err != nil && err != http.ErrServerClosed {
			log.Error("signal server error", zap.Error(err))
		}
	}()

	return nil
}

func (t *webrtcTransport) SignalPort() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.signalListener == nil {
		return 0
	}
	return t.signalListener.Addr().(*net.TCPAddr).Port
}

func (t *webrtcTransport) handleOffer(offerSDP string) (string, error) {
	pc, err := t.newPeerConnection()
	if err != nil {
		return "", fmt.Errorf("new peer connection: %w", err)
	}

	// Register OnDataChannel BEFORE SetRemoteDescription to avoid race
	dcCh := make(chan *webrtc.DataChannel, 1)
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		select {
		case dcCh <- dc:
		default:
		}
	})

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}
	if err = pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return "", fmt.Errorf("set remote desc: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		return "", fmt.Errorf("create answer: %w", err)
	}
	if err = pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return "", fmt.Errorf("set local desc: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherComplete:
	case <-time.After(time.Duration(t.conf.DialTimeoutSec) * time.Second):
		_ = pc.Close()
		return "", fmt.Errorf("ICE gathering timeout")
	}

	go func() {
		if err := t.acceptPeerConnection(pc, dcCh); err != nil {
			log.Info("accept peer connection error", zap.Error(err))
			_ = pc.Close()
		}
	}()

	return pc.LocalDescription().SDP, nil
}

func (t *webrtcTransport) acceptPeerConnection(pc *webrtc.PeerConnection, dcCh <-chan *webrtc.DataChannel) error {
	ctx, cancel := context.WithTimeout(t.listCtx, time.Duration(t.conf.DialTimeoutSec)*time.Second)
	defer cancel()

	// Wait for the handshake DataChannel
	var hsDC *webrtc.DataChannel
	var remotePeerId string
	select {
	case dc := <-dcCh:
		hsDC = dc
		label := dc.Label()
		if strings.HasPrefix(label, handshakeDCPrefix) {
			remotePeerId = label[len(handshakeDCPrefix):]
		}
	case <-ctx.Done():
		return fmt.Errorf("wait handshake dc: %w", ctx.Err())
	}

	// Wait for the DC to open
	if err := waitDCOpen(ctx, hsDC); err != nil {
		return fmt.Errorf("wait dc open: %w", err)
	}

	stream := wrapDataChannel(hsDC)
	defer stream.Close()

	// Application-level handshake verifies identity cryptographically
	cctx, err := t.secure.HandshakeInbound(ctx, stream, remotePeerId)
	if err != nil {
		return fmt.Errorf("handshake inbound: %w", err)
	}

	mc := newConn(cctx, pc, remotePeerId)
	return t.accepter.Accept(mc)
}

func (t *webrtcTransport) Dial(ctx context.Context, addr string) (mc transport.MultiConn, err error) {
	// addr format: host:signalPort
	signalURL := "http://" + addr + signalPath
	if idx := strings.Index(addr, "/certhash/"); idx != -1 {
		signalURL = "http://" + addr[:idx] + signalPath
		addr = addr[:idx]
	}

	expectedPeerId, _ := netpeer.CtxExpectedPeerId(ctx)

	pc, err := t.newPeerConnection()
	if err != nil {
		return nil, fmt.Errorf("new peer connection: %w", err)
	}

	// Create handshake DataChannel with local peerId in label
	ordered := true
	hsDC, err := pc.CreateDataChannel(handshakeDCPrefix+t.localPeerId, &webrtc.DataChannelInit{
		Ordered: &ordered,
	})
	if err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("create handshake dc: %w", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("create offer: %w", err)
	}
	if err = pc.SetLocalDescription(offer); err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("set local desc: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		_ = pc.Close()
		return nil, ctx.Err()
	}

	answer, err := signalExchange(ctx, signalURL, signalMessage{
		SDP:  pc.LocalDescription().SDP,
		Type: "offer",
	})
	if err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("signal exchange: %w", err)
	}

	if err = pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer.SDP,
	}); err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("set remote desc: %w", err)
	}

	if err = waitDCOpen(ctx, hsDC); err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("wait dc open: %w", err)
	}

	stream := wrapDataChannel(hsDC)
	defer stream.Close()

	remotePeerId := expectedPeerId
	if remotePeerId == "" {
		_ = pc.Close()
		return nil, fmt.Errorf("no expected peer id in context for WebRTC dial")
	}

	cctx, err := t.secure.HandshakeOutbound(ctx, stream, remotePeerId)
	if err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("handshake outbound: %w", err)
	}

	return newConn(cctx, pc, remotePeerId), nil
}

func (t *webrtcTransport) Close(ctx context.Context) error {
	if t.listCtxCancel != nil {
		t.listCtxCancel()
	}
	if t.signalServer != nil {
		_ = t.signalServer.Shutdown(ctx)
	}
	return nil
}

// waitDCOpen waits for a DataChannel to reach the Open state.
func waitDCOpen(ctx context.Context, dc *webrtc.DataChannel) error {
	if dc.ReadyState() == webrtc.DataChannelStateOpen {
		return nil
	}
	openCh := make(chan struct{}, 1)
	dc.OnOpen(func() {
		select {
		case openCh <- struct{}{}:
		default:
		}
	})
	if dc.ReadyState() == webrtc.DataChannelStateOpen {
		return nil
	}
	select {
	case <-openCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// wrapDataChannel wraps a *webrtc.DataChannel in a dcStream (net.Conn).
func wrapDataChannel(dc *webrtc.DataChannel) *dcStream {
	localAddr := webrtcAddr{addr: "local"}
	remoteAddr := webrtcAddr{addr: dc.Label()}

	raw, err := dc.Detach()
	if err != nil {
		return newDCStream(&pionDCWrapper{dc: dc}, localAddr, remoteAddr)
	}
	return newDCStream(&detachedDCWrapper{
		dc:  dc,
		rw:  raw,
		lbl: dc.Label(),
	}, localAddr, remoteAddr)
}
