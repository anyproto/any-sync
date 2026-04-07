//go:build js

package webtransport

import (
	"context"
	"fmt"
	"net/url"
	"runtime"
	"syscall/js"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	netpeer "github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
)

func New() WebTransport {
	return new(wtTransportJS)
}

type wtTransportJS struct {
	secure      secureservice.SecureService
	localPeerId string
	accepter    transport.Accepter
	conf        Config
}

func (t *wtTransportJS) Init(a *app.App) (err error) {
	t.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	account := a.MustComponent(accountservice.CName).(accountservice.Service)
	t.localPeerId = account.Account().PeerId
	if cg, ok := a.Component("config").(configGetter); ok {
		t.conf = cg.GetWebTransport()
	}
	if t.conf.Path == "" {
		t.conf.Path = "/webtransport"
	}
	if t.conf.DialTimeoutSec <= 0 {
		t.conf.DialTimeoutSec = 30
	}
	return nil
}

func (t *wtTransportJS) Name() string {
	return CName
}

func (t *wtTransportJS) SetAccepter(accepter transport.Accepter) {
	t.accepter = accepter
}

func (t *wtTransportJS) Run(ctx context.Context) error { return nil }

func (t *wtTransportJS) Close(ctx context.Context) error { return nil }

func (t *wtTransportJS) Dial(ctx context.Context, addr string) (transport.MultiConn, error) {
	expectedPeerId, err := netpeer.CtxExpectedPeerId(ctx)
	if err != nil {
		return nil, fmt.Errorf("expected peer id required for WebTransport WASM dial: %w", err)
	}

	dialURL := "https://" + addr + t.conf.Path + "?peerId=" + url.QueryEscape(t.localPeerId)

	wtConstructor := js.Global().Get("WebTransport")
	if wtConstructor.IsUndefined() {
		return nil, fmt.Errorf("WebTransport API not available in this browser")
	}
	wtObj := wtConstructor.New(dialURL)

	// Wait for the connection to be ready.
	readyPromise := wtObj.Get("ready")
	if _, err := awaitPromise(ctx, readyPromise); err != nil {
		wtObj.Call("close")
		return nil, fmt.Errorf("webtransport ready: %w", err)
	}

	// Create a bidirectional stream for the handshake.
	createPromise := wtObj.Call("createBidirectionalStream")
	bidiStream, err := awaitPromise(ctx, createPromise)
	if err != nil {
		wtObj.Call("close")
		return nil, fmt.Errorf("create bidi stream: %w", err)
	}

	hsConn := newJSStream(bidiStream, addr)
	defer hsConn.Close()

	cctx, err := t.secure.HandshakeOutbound(ctx, hsConn, expectedPeerId)
	if err != nil {
		wtObj.Call("close")
		return nil, fmt.Errorf("handshake outbound: %w", err)
	}

	// Prevent GC from collecting wtObj while goroutine was suspended
	// at await points (JS promises).
	runtime.KeepAlive(wtObj)

	return newJSConn(cctx, wtObj, addr), nil
}
