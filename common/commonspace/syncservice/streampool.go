package syncservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/libp2p/go-libp2p-core/sec"
	"storj.io/drpc/drpcctx"
	"sync"
)

var ErrEmptyPeer = errors.New("don't have such a peer")
var ErrStreamClosed = errors.New("stream is already closed")

const maxSimultaneousOperationsPerStream = 10

// StreamPool can be made generic to work with different streams
type StreamPool interface {
	AddAndReadStream(stream spacesyncproto.SpaceStream) (err error)
	HasStream(peerId string) bool
	SyncClient
	Close() (err error)
}

type SyncClient interface {
	SendSync(peerId string,
		message *spacesyncproto.ObjectSyncMessage,
		msgCheck func(syncMessage *spacesyncproto.ObjectSyncMessage) bool) (reply *spacesyncproto.ObjectSyncMessage, err error)
	SendAsync(peerId string, message *spacesyncproto.ObjectSyncMessage) (err error)
	BroadcastAsync(message *spacesyncproto.ObjectSyncMessage) (err error)
}

type MessageHandler func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error)

type responseWaiter struct {
	ch       chan *spacesyncproto.ObjectSyncMessage
	msgCheck func(message *spacesyncproto.ObjectSyncMessage) bool
}

type streamPool struct {
	sync.Mutex
	peerStreams    map[string]spacesyncproto.SpaceStream
	messageHandler MessageHandler
	wg             *sync.WaitGroup
	waiters        map[string]responseWaiter
	waitersMx      sync.Mutex
}

func newStreamPool(messageHandler MessageHandler) StreamPool {
	return &streamPool{
		peerStreams:    make(map[string]spacesyncproto.SpaceStream),
		messageHandler: messageHandler,
		wg:             &sync.WaitGroup{},
	}
}

func (s *streamPool) HasStream(peerId string) (res bool) {
	_, err := s.getStream(peerId)
	return err == nil
}

func (s *streamPool) SendSync(
	peerId string,
	message *spacesyncproto.ObjectSyncMessage,
	msgCheck func(syncMessage *spacesyncproto.ObjectSyncMessage) bool) (reply *spacesyncproto.ObjectSyncMessage, err error) {

	sendAndWait := func(waiter responseWaiter) (err error) {
		err = s.SendAsync(peerId, message)
		if err != nil {
			return
		}

		reply = <-waiter.ch
		return
	}

	key := fmt.Sprintf("%s.%s", peerId, message.TreeId)
	s.waitersMx.Lock()
	waiter, exists := s.waiters[key]
	if exists {
		s.waitersMx.Unlock()

		err = sendAndWait(waiter)
		return
	}

	waiter = responseWaiter{
		ch:       make(chan *spacesyncproto.ObjectSyncMessage),
		msgCheck: msgCheck,
	}
	s.waiters[key] = waiter
	s.waitersMx.Unlock()
	err = sendAndWait(waiter)
	return
}

func (s *streamPool) SendAsync(peerId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	stream, err := s.getStream(peerId)
	if err != nil {
		return
	}

	return stream.Send(message)
}

func (s *streamPool) getStream(id string) (stream spacesyncproto.SpaceStream, err error) {
	s.Lock()
	defer s.Unlock()
	stream, exists := s.peerStreams[id]
	if !exists {
		err = ErrEmptyPeer
		return
	}

	select {
	case <-stream.Context().Done():
		delete(s.peerStreams, id)
		err = ErrStreamClosed
	default:
	}

	return
}

func (s *streamPool) getAllStreams() (streams []spacesyncproto.SpaceStream) {
	s.Lock()
	defer s.Unlock()
Loop:
	for id, stream := range s.peerStreams {
		select {
		case <-stream.Context().Done():
			delete(s.peerStreams, id)
			continue Loop
		default:
		}
		streams = append(streams, stream)
	}

	return
}

func (s *streamPool) BroadcastAsync(message *spacesyncproto.ObjectSyncMessage) (err error) {
	streams := s.getAllStreams()
	for _, stream := range streams {
		if err = stream.Send(message); err != nil {
			// TODO: add logging
		}
	}

	return nil
}

func (s *streamPool) AddAndReadStream(stream spacesyncproto.SpaceStream) (err error) {
	s.Lock()
	peerId, err := GetPeerIdFromStreamContext(stream.Context())
	if err != nil {
		s.Unlock()
		return
	}

	s.peerStreams[peerId] = stream
	s.wg.Add(1)
	s.Unlock()

	return s.readPeerLoop(peerId, stream)
}

func (s *streamPool) Close() (err error) {
	s.Lock()
	wg := s.wg
	s.Unlock()
	if wg != nil {
		wg.Wait()
	}
	return nil
}

func (s *streamPool) readPeerLoop(peerId string, stream spacesyncproto.SpaceStream) (err error) {
	defer s.wg.Done()
	limiter := make(chan struct{}, maxSimultaneousOperationsPerStream)
	for i := 0; i < maxSimultaneousOperationsPerStream; i++ {
		limiter <- struct{}{}
	}

	process := func(msg *spacesyncproto.ObjectSyncMessage) {
		key := fmt.Sprintf("%s.%s", peerId, msg.TreeId)
		s.waitersMx.Lock()
		waiter, exists := s.waiters[key]

		if !exists || !waiter.msgCheck(msg) {
			s.waitersMx.Unlock()
			s.messageHandler(stream.Context(), peerId, msg)
			return
		}

		delete(s.waiters, key)
		s.waitersMx.Unlock()
		waiter.ch <- msg
	}

Loop:
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}
		select {
		case <-limiter:
		case <-stream.Context().Done():
			break Loop
		}
		go func() {
			defer func() {
				limiter <- struct{}{}
			}()
			process(msg)
		}()
	}
	return s.removePeer(peerId)
}

func (s *streamPool) removePeer(peerId string) (err error) {
	s.Lock()
	defer s.Unlock()
	_, ok := s.peerStreams[peerId]
	if !ok {
		return ErrEmptyPeer
	}
	delete(s.peerStreams, peerId)
	return
}

func GetPeerIdFromStreamContext(ctx context.Context) (string, error) {
	conn, ok := ctx.Value(drpcctx.TransportKey{}).(sec.SecureConn)
	if !ok {
		return "", fmt.Errorf("incorrect connection type in stream")
	}

	return conn.RemotePeer().String(), nil
}
