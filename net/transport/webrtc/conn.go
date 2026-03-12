package webrtc

import (
	"context"
	"io"
	"net"
	"runtime"
	"sync"

	"github.com/pion/webrtc/v4"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

// prevent Go GC from collecting active PeerConnections/DataChannels and their
// js.Func handlers while JavaScript still holds references to them.
var (
	activePCs   = make(map[*webrtcMultiConn]struct{})
	activeDCs   = make(map[*webrtc.DataChannel]struct{})
	activeJSMu  sync.Mutex
)

func newConn(cctx context.Context, pc *webrtc.PeerConnection, remoteAddr string) transport.MultiConn {
	cctx = peer.CtxWithPeerAddr(cctx, transport.WebRTC+"://"+remoteAddr)
	closeCh := make(chan struct{})
	mc := &webrtcMultiConn{
		cctx:       cctx,
		pc:         pc,
		incomingDC: make(chan *webrtc.DataChannel, 64),
		closeCh:    closeCh,
		remoteAddr: remoteAddr,
	}

	// Pin mc so GC cannot collect it (and its pc with js.Func handlers)
	activeJSMu.Lock()
	activePCs[mc] = struct{}{}
	activeJSMu.Unlock()

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		select {
		case mc.incomingDC <- dc:
		case <-closeCh:
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed,
			webrtc.PeerConnectionStateDisconnected:
			mc.closeOnce.Do(func() { close(closeCh) })
		}
	})

	return mc
}

type webrtcMultiConn struct {
	cctx       context.Context
	pc         *webrtc.PeerConnection
	incomingDC chan *webrtc.DataChannel
	closeCh    chan struct{}
	closeOnce  sync.Once
	remoteAddr string
}

func (m *webrtcMultiConn) Context() context.Context {
	return m.cctx
}

func (m *webrtcMultiConn) Accept() (conn net.Conn, err error) {
	select {
	case dc := <-m.incomingDC:
		return m.wrapDC(dc), nil
	case <-m.closeCh:
		return nil, transport.ErrConnClosed
	}
}

func (m *webrtcMultiConn) Open(ctx context.Context) (conn net.Conn, err error) {
	ordered := true
	dc, err := m.pc.CreateDataChannel("", &webrtc.DataChannelInit{
		Ordered: &ordered,
	})
	if err != nil {
		return nil, err
	}

	// Wait for datachannel to open
	openCh := make(chan struct{})
	dc.OnOpen(func() {
		close(openCh)
	})
	select {
	case <-openCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.closeCh:
		return nil, transport.ErrConnClosed
	}

	// Prevent GC from collecting dc (and its js.Func handlers) while
	// the goroutine was suspended waiting for the DC to open.
	runtime.KeepAlive(dc)

	return m.wrapDC(dc), nil
}

func (m *webrtcMultiConn) wrapDC(dc *webrtc.DataChannel) net.Conn {
	// Pin DC so GC cannot collect its js.Func handlers
	activeJSMu.Lock()
	activeDCs[dc] = struct{}{}
	activeJSMu.Unlock()

	localAddr := webrtcAddr{addr: "local"}
	remoteAddr := webrtcAddr{addr: m.remoteAddr}
	raw, err := dc.Detach()
	if err != nil {
		// Fallback: use the non-detached API through adapter
		return newDCStream(&pionDCWrapper{dc: dc}, localAddr, remoteAddr)
	}
	return newDCStream(&detachedDCWrapper{
		dc:  dc,
		rw:  raw,
		lbl: dc.Label(),
	}, localAddr, remoteAddr)
}

func (m *webrtcMultiConn) Addr() string {
	return transport.WebRTC + "://" + m.remoteAddr
}

func (m *webrtcMultiConn) IsClosed() bool {
	select {
	case <-m.closeCh:
		return true
	default:
		return false
	}
}

func (m *webrtcMultiConn) CloseChan() <-chan struct{} {
	return m.closeCh
}

func (m *webrtcMultiConn) Close() error {
	m.closeOnce.Do(func() { close(m.closeCh) })
	activeJSMu.Lock()
	delete(activePCs, m)
	activeJSMu.Unlock()
	clearPCHandlers(m.pc)
	return m.pc.Close()
}

// webrtcAddr implements net.Addr for WebRTC connections.
type webrtcAddr struct {
	addr string
}

func (a webrtcAddr) Network() string { return "webrtc" }
func (a webrtcAddr) String() string  { return a.addr }

// pionDCWrapper adapts *webrtc.DataChannel to the dataChannel interface
// when detach is not available.
type pionDCWrapper struct {
	dc        *webrtc.DataChannel
	onMsg     func(msg []byte)
	onClose   func()
}

func (w *pionDCWrapper) Send(data []byte) error {
	return w.dc.Send(data)
}

func (w *pionDCWrapper) SetOnMessage(f func(msg []byte)) {
	w.onMsg = f
	w.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		f(msg.Data)
	})
}

func (w *pionDCWrapper) SetOnClose(f func()) {
	w.onClose = f
	w.dc.OnClose(f)
}

func (w *pionDCWrapper) Close() error {
	activeJSMu.Lock()
	delete(activeDCs, w.dc)
	activeJSMu.Unlock()
	clearDCHandlers(w.dc)
	return w.dc.Close()
}

func (w *pionDCWrapper) Label() string {
	return w.dc.Label()
}

// detachedDCWrapper adapts a detached DataChannel (datachannel.ReadWriteCloser)
// to the dataChannel interface. With detach, we get raw bytes without the
// pion callback overhead.
type detachedDCWrapper struct {
	dc     *webrtc.DataChannel
	rw     io.ReadWriteCloser
	lbl    string
	mu     sync.Mutex
	closed bool
}

func (w *detachedDCWrapper) Send(data []byte) error {
	_, err := w.rw.Write(data)
	return err
}

func (w *detachedDCWrapper) SetOnMessage(f func(msg []byte)) {
	// With detach mode, we need to read in a goroutine
	go func() {
		buf := make([]byte, maxPayload+headerSize)
		for {
			n, err := w.rw.Read(buf)
			if n > 0 {
				msg := make([]byte, n)
				copy(msg, buf[:n])
				f(msg)
			}
			if err != nil {
				return
			}
		}
	}()
}

func (w *detachedDCWrapper) SetOnClose(f func()) {
	w.dc.OnClose(f)
}

func (w *detachedDCWrapper) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	activeJSMu.Lock()
	delete(activeDCs, w.dc)
	activeJSMu.Unlock()
	_ = w.rw.Close()
	clearDCHandlers(w.dc)
	return w.dc.Close()
}

func (w *detachedDCWrapper) Label() string {
	return w.lbl
}

