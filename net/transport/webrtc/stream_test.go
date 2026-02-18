package webrtc

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDataChannel is a simple in-memory DataChannel for testing.
type mockDataChannel struct {
	mu      sync.Mutex
	onMsg   func([]byte)
	onClose func()
	peer    *mockDataChannel
	closed  bool
	lbl     string
}

func newMockDCPair(label string) (*mockDataChannel, *mockDataChannel) {
	a := &mockDataChannel{lbl: label}
	b := &mockDataChannel{lbl: label}
	a.peer = b
	b.peer = a
	return a, b
}

func (m *mockDataChannel) Send(data []byte) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errStreamClosed
	}
	peer := m.peer
	m.mu.Unlock()

	if peer == nil {
		return errStreamClosed
	}
	peer.mu.Lock()
	onMsg := peer.onMsg
	peer.mu.Unlock()
	if onMsg != nil {
		// Copy data to avoid race
		msg := make([]byte, len(data))
		copy(msg, data)
		onMsg(msg)
	}
	return nil
}

func (m *mockDataChannel) SetOnMessage(f func([]byte)) {
	m.mu.Lock()
	m.onMsg = f
	m.mu.Unlock()
}

func (m *mockDataChannel) SetOnClose(f func()) {
	m.mu.Lock()
	m.onClose = f
	m.mu.Unlock()
}

func (m *mockDataChannel) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	onClose := m.onClose
	m.mu.Unlock()
	if onClose != nil {
		onClose()
	}
	return nil
}

func (m *mockDataChannel) Label() string {
	return m.lbl
}

func newTestStreamPair(t *testing.T) (*dcStream, *dcStream) {
	dcA, dcB := newMockDCPair("test")
	addr := webrtcAddr{addr: "test"}
	sA := newDCStream(dcA, addr, addr)
	sB := newDCStream(dcB, addr, addr)
	return sA, sB
}

func TestDCStream_WriteRead(t *testing.T) {
	sA, sB := newTestStreamPair(t)

	data := []byte("hello world")
	n, err := sA.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	buf := make([]byte, 64)
	n, err = sB.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, data, buf[:n])
}

func TestDCStream_WriteReadLarge(t *testing.T) {
	sA, sB := newTestStreamPair(t)

	// Write more than maxPayload to test fragmentation
	data := make([]byte, maxPayload*2+100)
	for i := range data {
		data[i] = byte(i % 256)
	}
	n, err := sA.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Read all fragments
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	for buf.Len() < len(data) {
		rn, rerr := sB.Read(tmp)
		require.NoError(t, rerr)
		buf.Write(tmp[:rn])
	}
	assert.Equal(t, data, buf.Bytes())
}

func TestDCStream_HalfClose(t *testing.T) {
	sA, sB := newTestStreamPair(t)

	// A writes data and closes
	data := []byte("half close test")
	_, err := sA.Write(data)
	require.NoError(t, err)
	require.NoError(t, sA.Close())

	// B should be able to read the data
	buf := make([]byte, 64)
	n, err := sB.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, data, buf[:n])

	// B should get EOF after data is consumed
	_, err = sB.Read(buf)
	assert.ErrorIs(t, err, io.EOF)
}

func TestDCStream_Bidirectional(t *testing.T) {
	sA, sB := newTestStreamPair(t)

	// Both sides write
	_, err := sA.Write([]byte("from A"))
	require.NoError(t, err)
	_, err = sB.Write([]byte("from B"))
	require.NoError(t, err)

	// Both sides read
	bufA := make([]byte, 64)
	nA, err := sA.Read(bufA)
	require.NoError(t, err)
	assert.Equal(t, "from B", string(bufA[:nA]))

	bufB := make([]byte, 64)
	nB, err := sB.Read(bufB)
	require.NoError(t, err)
	assert.Equal(t, "from A", string(bufB[:nB]))

	// Close both independently
	require.NoError(t, sA.Close())
	require.NoError(t, sB.Close())
}

func TestDCStream_RST(t *testing.T) {
	sA, sB := newTestStreamPair(t)

	// Send RST from A
	err := sA.dc.Send([]byte{frameRST})
	require.NoError(t, err)

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	// B should get reset error on Read
	buf := make([]byte, 64)
	_, err = sB.Read(buf)
	assert.ErrorIs(t, err, errStreamReset)
}

func TestDCStream_ConcurrentReadWrite(t *testing.T) {
	sA, sB := newTestStreamPair(t)

	var wg sync.WaitGroup
	messages := 100

	// Writer goroutine on A
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < messages; i++ {
			_, err := sA.Write([]byte("msg"))
			if err != nil {
				return
			}
		}
		sA.Close()
	}()

	// Reader goroutine on B
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 64)
		count := 0
		for {
			n, err := sB.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				count++
			}
		}
	}()

	wg.Wait()
}

func TestDCStream_WriteAfterClose(t *testing.T) {
	sA, _ := newTestStreamPair(t)

	require.NoError(t, sA.Close())
	_, err := sA.Write([]byte("should fail"))
	assert.ErrorIs(t, err, errStreamClosed)
}

func TestDCStream_DoubleClose(t *testing.T) {
	sA, _ := newTestStreamPair(t)
	require.NoError(t, sA.Close())
	require.NoError(t, sA.Close())
}

func TestFrameEncoding(t *testing.T) {
	payload := []byte("test payload")
	frame := make([]byte, headerSize+len(payload))
	frame[0] = frameData
	binary.LittleEndian.PutUint16(frame[1:3], uint16(len(payload)))
	copy(frame[headerSize:], payload)

	assert.Equal(t, frameData, frame[0])
	assert.Equal(t, uint16(len(payload)), binary.LittleEndian.Uint16(frame[1:3]))
	assert.Equal(t, payload, frame[headerSize:])
}

func TestDCStream_EmptyRead(t *testing.T) {
	sA, sB := newTestStreamPair(t)

	// Write empty payload (edge case)
	_, err := sA.Write([]byte{})
	require.NoError(t, err)

	// Write actual data
	_, err = sA.Write([]byte("data"))
	require.NoError(t, err)

	buf := make([]byte, 64)
	n, err := sB.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "data", string(buf[:n]))
}
