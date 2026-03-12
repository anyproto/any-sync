//go:build js

package webtransport

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall/js"
	"time"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

// awaitPromise blocks until a JS promise resolves or the context is cancelled.
func awaitPromise(ctx context.Context, promise js.Value) (js.Value, error) {
	type promiseResult struct {
		val js.Value
		err error
	}
	ch := make(chan promiseResult, 1)

	thenFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		val := js.Undefined()
		if len(args) > 0 {
			val = args[0]
		}
		select {
		case ch <- promiseResult{val: val}:
		default:
		}
		return nil
	})

	catchFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		msg := "promise rejected"
		if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
			msg = args[0].Call("toString").String()
		}
		select {
		case ch <- promiseResult{err: fmt.Errorf("%s", msg)}:
		default:
		}
		return nil
	})

	promise.Call("then", thenFunc).Call("catch", catchFunc)

	select {
	case r := <-ch:
		thenFunc.Release()
		catchFunc.Release()
		return r.val, r.err
	case <-ctx.Done():
		// Release funcs asynchronously after the promise settles
		// to avoid "call to released function" panics.
		go func() {
			<-ch
			thenFunc.Release()
			catchFunc.Release()
		}()
		return js.Value{}, ctx.Err()
	}
}

// jsStream wraps a browser WebTransportBidirectionalStream as a net.Conn.
type jsStream struct {
	reader     js.Value // ReadableStreamDefaultReader
	writer     js.Value // WritableStreamDefaultWriter
	localAddr  net.Addr
	remoteAddr net.Addr
	readBuf    []byte
	readMu     sync.Mutex
	writeMu    sync.Mutex
	closeOnce  sync.Once
	closeCtx   context.Context
	closeFunc  context.CancelFunc
}

func newJSStream(bidiStream js.Value, remoteAddr string) *jsStream {
	readable := bidiStream.Get("readable")
	writable := bidiStream.Get("writable")
	reader := readable.Call("getReader")
	writer := writable.Call("getWriter")
	ctx, cancel := context.WithCancel(context.Background())
	return &jsStream{
		reader:     reader,
		writer:     writer,
		localAddr:  wtAddr{addr: "local"},
		remoteAddr: wtAddr{addr: remoteAddr},
		closeCtx:   ctx,
		closeFunc:  cancel,
	}
}

func (s *jsStream) Read(b []byte) (int, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()

	if s.closeCtx.Err() != nil {
		return 0, io.EOF
	}

	// Return buffered data first.
	if len(s.readBuf) > 0 {
		n := copy(b, s.readBuf)
		s.readBuf = s.readBuf[n:]
		return n, nil
	}

	readPromise := s.reader.Call("read")
	result, err := awaitPromise(s.closeCtx, readPromise)
	if err != nil {
		return 0, err
	}

	done := result.Get("done").Bool()
	if done {
		return 0, io.EOF
	}

	value := result.Get("value")
	dataLen := value.Get("byteLength").Int()
	data := make([]byte, dataLen)
	js.CopyBytesToGo(data, value)

	n := copy(b, data)
	if n < dataLen {
		s.readBuf = data[n:]
	}
	return n, nil
}

func (s *jsStream) Write(b []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.closeCtx.Err() != nil {
		return 0, io.ErrClosedPipe
	}

	uint8Array := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(uint8Array, b)

	writePromise := s.writer.Call("write", uint8Array)
	if _, err := awaitPromise(s.closeCtx, writePromise); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (s *jsStream) Close() error {
	s.closeOnce.Do(func() {
		s.closeFunc()
		s.reader.Call("cancel")
		// Use abort() instead of close() because close() returns a rejected
		// promise if the writable stream is already errored (e.g. the remote
		// side closed first), causing an unhandled promise rejection.
		s.writer.Call("abort")
	})
	return nil
}

func (s *jsStream) LocalAddr() net.Addr                { return s.localAddr }
func (s *jsStream) RemoteAddr() net.Addr               { return s.remoteAddr }
func (s *jsStream) SetDeadline(t time.Time) error      { return nil }
func (s *jsStream) SetReadDeadline(t time.Time) error  { return nil }
func (s *jsStream) SetWriteDeadline(t time.Time) error { return nil }

// jsMultiConn wraps a browser WebTransport object as transport.MultiConn.
type jsMultiConn struct {
	cctx           context.Context
	wt             js.Value // the WebTransport JS object
	incomingReader js.Value // ReadableStreamDefaultReader for incoming bidi streams
	closeCh        chan struct{}
	closeOnce      sync.Once
	remoteAddr     string
}

func newJSConn(cctx context.Context, wtObj js.Value, remoteAddr string) transport.MultiConn {
	cctx = peer.CtxWithPeerAddr(cctx, transport.WebTransport+"://"+remoteAddr)
	closeCh := make(chan struct{})

	incoming := wtObj.Get("incomingBidirectionalStreams")
	incomingReader := incoming.Call("getReader")

	mc := &jsMultiConn{
		cctx:           cctx,
		wt:             wtObj,
		incomingReader: incomingReader,
		closeCh:        closeCh,
		remoteAddr:     remoteAddr,
	}

	// Watch for connection close.
	closedPromise := wtObj.Get("closed")
	go func() {
		awaitPromise(context.Background(), closedPromise)
		mc.closeOnce.Do(func() { close(closeCh) })
	}()

	return mc
}

func (m *jsMultiConn) Context() context.Context {
	return m.cctx
}

func (m *jsMultiConn) Accept() (net.Conn, error) {
	readPromise := m.incomingReader.Call("read")
	result, err := awaitPromise(context.Background(), readPromise)
	if err != nil {
		return nil, err
	}
	done := result.Get("done").Bool()
	if done {
		return nil, transport.ErrConnClosed
	}
	stream := result.Get("value")
	return newJSStream(stream, m.remoteAddr), nil
}

func (m *jsMultiConn) Open(ctx context.Context) (net.Conn, error) {
	createPromise := m.wt.Call("createBidirectionalStream")
	stream, err := awaitPromise(ctx, createPromise)
	if err != nil {
		return nil, err
	}
	return newJSStream(stream, m.remoteAddr), nil
}

func (m *jsMultiConn) Addr() string {
	return transport.WebTransport + "://" + m.remoteAddr
}

func (m *jsMultiConn) IsClosed() bool {
	select {
	case <-m.closeCh:
		return true
	default:
		return false
	}
}

func (m *jsMultiConn) CloseChan() <-chan struct{} {
	return m.closeCh
}

func (m *jsMultiConn) Close() error {
	m.closeOnce.Do(func() { close(m.closeCh) })
	m.wt.Call("close")
	return nil
}
