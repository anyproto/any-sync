package webrtc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// Frame types for the DataChannel framing protocol.
// Browser RTCDataChannel.close() kills both read and write sides.
// DRPC expects net.Conn half-close semantics, so we implement a FIN/FIN_ACK
// framing layer on top of DataChannel messages.
const (
	frameData   byte = 0x01
	frameFIN    byte = 0x02
	frameFINACK byte = 0x03
	frameRST    byte = 0x04
)

// maxPayload is the maximum payload size per DATA frame.
// Kept under the 16 KiB DataChannel message size limit.
const maxPayload = 16128

// headerSize is the size of the frame header (1 byte type + 2 bytes length).
const headerSize = 3

var (
	errStreamClosed = errors.New("stream closed")
	errStreamReset  = errors.New("stream reset by remote")
)

// dataChannel is the interface needed from a pion DataChannel.
// Abstracted for testability and to support both native and WASM.
type dataChannel interface {
	Send(data []byte) error
	SetOnMessage(f func(msg []byte))
	SetOnClose(f func())
	Close() error
	Label() string
}

// dcStream wraps a DataChannel to provide net.Conn semantics with half-close
// support via a simple framing protocol.
type dcStream struct {
	dc dataChannel

	readBuf  *bytes.Buffer
	readCond *sync.Cond

	mu        sync.Mutex // protects state below
	localFIN  bool
	remoteFIN bool
	finAckCh  chan struct{}
	remoteRST bool

	localAddr  net.Addr
	remoteAddr net.Addr
	closeCh    chan struct{}
	closeOnce  sync.Once

	readDeadline  time.Time
	writeDeadline time.Time
}

func newDCStream(dc dataChannel, localAddr, remoteAddr net.Addr) *dcStream {
	mu := &sync.Mutex{}
	s := &dcStream{
		dc:       dc,
		readBuf:  bytes.NewBuffer(nil),
		readCond: sync.NewCond(mu),
		finAckCh: make(chan struct{}),
		closeCh:  make(chan struct{}),

		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}

	dc.SetOnMessage(s.onMessage)
	dc.SetOnClose(func() {
		s.closeOnce.Do(func() { close(s.closeCh) })
	})

	return s
}

func (s *dcStream) onMessage(msg []byte) {
	if len(msg) < 1 {
		return
	}
	frameType := msg[0]
	switch frameType {
	case frameData:
		if len(msg) < headerSize {
			return
		}
		payloadLen := int(binary.LittleEndian.Uint16(msg[1:3]))
		if len(msg) < headerSize+payloadLen {
			return
		}
		payload := msg[headerSize : headerSize+payloadLen]
		s.readCond.L.Lock()
		s.readBuf.Write(payload)
		s.readCond.Broadcast()
		s.readCond.L.Unlock()
	case frameFIN:
		s.readCond.L.Lock()
		s.remoteFIN = true
		s.readCond.Broadcast()
		s.readCond.L.Unlock()
		// send FIN_ACK
		_ = s.dc.Send([]byte{frameFINACK})
		s.tryCloseUnderlying()
	case frameFINACK:
		s.mu.Lock()
		select {
		case <-s.finAckCh:
		default:
			close(s.finAckCh)
		}
		s.mu.Unlock()
		s.tryCloseUnderlying()
	case frameRST:
		s.readCond.L.Lock()
		s.remoteRST = true
		s.remoteFIN = true
		s.readCond.Broadcast()
		s.readCond.L.Unlock()
		s.closeOnce.Do(func() { close(s.closeCh) })
		_ = s.dc.Close()
	}
}

// tryCloseUnderlying closes the DataChannel if both sides have FIN'd
// and we received FIN_ACK for our FIN.
func (s *dcStream) tryCloseUnderlying() {
	s.readCond.L.Lock()
	remoteFIN := s.remoteFIN
	s.readCond.L.Unlock()

	s.mu.Lock()
	localFIN := s.localFIN
	finAcked := false
	select {
	case <-s.finAckCh:
		finAcked = true
	default:
	}
	s.mu.Unlock()

	if localFIN && remoteFIN && finAcked {
		s.closeOnce.Do(func() { close(s.closeCh) })
		_ = s.dc.Close()
	}
}

func (s *dcStream) Read(b []byte) (n int, err error) {
	s.readCond.L.Lock()
	defer s.readCond.L.Unlock()

	for s.readBuf.Len() == 0 {
		if s.remoteRST {
			return 0, errStreamReset
		}
		if s.remoteFIN {
			return 0, io.EOF
		}
		select {
		case <-s.closeCh:
			return 0, errStreamClosed
		default:
		}
		s.readCond.Wait()
	}
	return s.readBuf.Read(b)
}

func (s *dcStream) Write(b []byte) (n int, err error) {
	s.mu.Lock()
	if s.localFIN {
		s.mu.Unlock()
		return 0, errStreamClosed
	}
	s.mu.Unlock()

	select {
	case <-s.closeCh:
		return 0, errStreamClosed
	default:
	}

	for n < len(b) {
		end := n + maxPayload
		if end > len(b) {
			end = len(b)
		}
		chunk := b[n:end]
		frame := make([]byte, headerSize+len(chunk))
		frame[0] = frameData
		binary.LittleEndian.PutUint16(frame[1:3], uint16(len(chunk)))
		copy(frame[headerSize:], chunk)
		if err = s.dc.Send(frame); err != nil {
			return n, err
		}
		n += len(chunk)
	}
	return n, nil
}

// Close sends FIN and stops the write side. Reads still work until the remote
// side also FINs.
func (s *dcStream) Close() error {
	s.mu.Lock()
	if s.localFIN {
		s.mu.Unlock()
		return nil
	}
	s.localFIN = true
	s.mu.Unlock()

	_ = s.dc.Send([]byte{frameFIN})
	s.tryCloseUnderlying()
	return nil
}

func (s *dcStream) LocalAddr() net.Addr  { return s.localAddr }
func (s *dcStream) RemoteAddr() net.Addr { return s.remoteAddr }

func (s *dcStream) SetDeadline(t time.Time) error {
	_ = s.SetReadDeadline(t)
	_ = s.SetWriteDeadline(t)
	return nil
}

func (s *dcStream) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	s.readDeadline = t
	s.mu.Unlock()
	return nil
}

func (s *dcStream) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	s.writeDeadline = t
	s.mu.Unlock()
	return nil
}
