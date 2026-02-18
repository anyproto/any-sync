//go:build js

package webrtc

import (
	"context"
	"fmt"

	"github.com/pion/webrtc/v4"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	netpeer "github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
)

func New() WebRTC {
	return new(webrtcTransportJS)
}

type webrtcTransportJS struct {
	secure      secureservice.SecureService
	localPeerId string
	accepter    transport.Accepter
	conf        Config
}

func (t *webrtcTransportJS) Init(a *app.App) (err error) {
	t.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	account := a.MustComponent(accountservice.CName).(accountservice.Service)
	t.localPeerId = account.Account().PeerId
	if cg, ok := a.Component("config").(configGetter); ok {
		t.conf = cg.GetWebRTC()
	}
	if t.conf.DialTimeoutSec <= 0 {
		t.conf.DialTimeoutSec = 30
	}
	return nil
}

func (t *webrtcTransportJS) Name() string {
	return CName
}

func (t *webrtcTransportJS) SetAccepter(accepter transport.Accepter) {
	t.accepter = accepter
}

func (t *webrtcTransportJS) Dial(ctx context.Context, addr string) (mc transport.MultiConn, err error) {
	signalURL := "http://" + addr + signalPath

	expectedPeerId, err := netpeer.CtxExpectedPeerId(ctx)
	if err != nil {
		return nil, fmt.Errorf("expected peer id required for WebRTC WASM dial: %w", err)
	}

	// In WASM, pion/webrtc v4 uses the browser's RTCPeerConnection
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, fmt.Errorf("new peer connection: %w", err)
	}

	// Create handshake DataChannel with LOCAL peerId in label
	// so the server knows who the client is
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

	// Wait for handshake DC to open
	openCh := make(chan struct{}, 1)
	hsDC.OnOpen(func() {
		select {
		case openCh <- struct{}{}:
		default:
		}
	})
	if hsDC.ReadyState() == webrtc.DataChannelStateOpen {
		select {
		case openCh <- struct{}{}:
		default:
		}
	}
	select {
	case <-openCh:
	case <-ctx.Done():
		_ = pc.Close()
		return nil, ctx.Err()
	}

	// Wrap DC as net.Conn for the handshake
	localAddr := webrtcAddr{addr: "local"}
	remoteAddr := webrtcAddr{addr: expectedPeerId}
	stream := newDCStream(&pionDCWrapper{dc: hsDC}, localAddr, remoteAddr)
	defer stream.Close()

	// Application handshake verifies server identity cryptographically
	cctx, err := t.secure.HandshakeOutbound(ctx, stream, expectedPeerId)
	if err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("handshake outbound: %w", err)
	}

	return newConn(cctx, pc, expectedPeerId), nil
}
