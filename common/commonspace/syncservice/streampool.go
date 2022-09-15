package syncservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/libp2p/go-libp2p-core/sec"
	"storj.io/drpc"
	"storj.io/drpc/drpcctx"
	"sync"
)

var ErrEmptyPeer = errors.New("don't have such a peer")
var ErrStreamClosed = errors.New("stream is already closed")

const maxSimultaneousOperationsPerStream = 10

// StreamPool can be made generic to work with different streams
type StreamPool interface {
	AddStream(stream spacesyncproto.SpaceStream) (err error)
	HasStream(peerId string) bool
	SyncClient
}

type SyncClient interface {
	SendAsync(peerId string, message *spacesyncproto.ObjectSyncMessage) (err error)
	BroadcastAsync(message *spacesyncproto.ObjectSyncMessage) (err error)
}

type MessageHandler func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error)

type streamPool struct {
	sync.Mutex
	peerStreams    map[string]spacesyncproto.SpaceStream
	messageHandler MessageHandler
}

func newStreamPool(messageHandler MessageHandler) StreamPool {
	return &streamPool{
		peerStreams:    make(map[string]spacesyncproto.SpaceStream),
		messageHandler: messageHandler,
	}
}

func (s *streamPool) HasStream(peerId string) (res bool) {
	_, err := s.getStream(peerId)
	return err == nil
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

func (s *streamPool) AddStream(stream spacesyncproto.SpaceStream) (err error) {
	s.Lock()
	peerId, err := getPeerIdFromStream(stream)
	if err != nil {
		s.Unlock()
		return
	}

	s.peerStreams[peerId] = stream
	s.Unlock()

	return s.readPeerLoop(peerId, stream)
}

func (s *streamPool) readPeerLoop(peerId string, stream spacesyncproto.SpaceStream) (err error) {
	limiter := make(chan struct{}, maxSimultaneousOperationsPerStream)
	for i := 0; i < maxSimultaneousOperationsPerStream; i++ {
		limiter <- struct{}{}
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

			s.messageHandler(context.Background(), peerId, msg)
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

func getPeerIdFromStream(stream drpc.Stream) (string, error) {
	ctx := stream.Context()
	conn, ok := ctx.Value(drpcctx.TransportKey{}).(sec.SecureConn)
	if !ok {
		return "", fmt.Errorf("incorrect connection type in stream")
	}

	return conn.RemotePeer().String(), nil
}
