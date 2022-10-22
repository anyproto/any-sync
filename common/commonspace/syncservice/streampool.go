package syncservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"github.com/libp2p/go-libp2p-core/sec"
	"storj.io/drpc/drpcctx"
	"sync"
	"sync/atomic"
	"time"
)

var ErrEmptyPeer = errors.New("don't have such a peer")
var ErrStreamClosed = errors.New("stream is already closed")

var maxSimultaneousOperationsPerStream = 10
var syncWaitPeriod = 2 * time.Second

var ErrSyncTimeout = errors.New("too long wait on sync receive")

// StreamPool can be made generic to work with different streams
type StreamPool interface {
	ocache.ObjectLastUsage
	AddAndReadStreamSync(stream spacesyncproto.SpaceStream) (err error)
	AddAndReadStreamAsync(stream spacesyncproto.SpaceStream)

	SendSync(peerId string, message *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
	SendAsync(peers []string, message *spacesyncproto.ObjectSyncMessage) (err error)
	BroadcastAsync(message *spacesyncproto.ObjectSyncMessage) (err error)

	HasActiveStream(peerId string) bool
	Close() (err error)
}

type MessageHandler func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error)

type responseWaiter struct {
	ch chan *spacesyncproto.ObjectSyncMessage
}

type streamPool struct {
	sync.Mutex
	peerStreams    map[string]spacesyncproto.SpaceStream
	messageHandler MessageHandler
	wg             *sync.WaitGroup
	waiters        map[string]responseWaiter
	waitersMx      sync.Mutex
	counter        atomic.Uint64
	lastUsage      atomic.Int64
}

func newStreamPool(messageHandler MessageHandler) StreamPool {
	s := &streamPool{
		peerStreams:    make(map[string]spacesyncproto.SpaceStream),
		messageHandler: messageHandler,
		waiters:        make(map[string]responseWaiter),
		wg:             &sync.WaitGroup{},
	}
	s.lastUsage.Store(time.Now().Unix())
	return s
}

func (s *streamPool) LastUsage() time.Time {
	return time.Unix(s.lastUsage.Load(), 0)
}

func (s *streamPool) HasActiveStream(peerId string) (res bool) {
	s.Lock()
	defer s.Unlock()
	_, err := s.getOrDeleteStream(peerId)
	return err == nil
}

func (s *streamPool) SendSync(
	peerId string,
	msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	newCounter := s.counter.Add(1)
	msg.ReplyId = genStreamPoolKey(peerId, msg.ObjectId, newCounter)

	s.waitersMx.Lock()
	waiter := responseWaiter{
		ch: make(chan *spacesyncproto.ObjectSyncMessage, 1),
	}
	s.waiters[msg.ReplyId] = waiter
	s.waitersMx.Unlock()

	err = s.SendAsync([]string{peerId}, msg)
	if err != nil {
		return
	}
	delay := time.NewTimer(syncWaitPeriod)
	select {
	case <-delay.C:
		s.waitersMx.Lock()
		delete(s.waiters, msg.ReplyId)
		s.waitersMx.Unlock()

		log.With("trackingId", msg.ReplyId).Error("time elapsed when waiting")
		err = ErrSyncTimeout
	case reply = <-waiter.ch:
		if !delay.Stop() {
			<-delay.C
		}
	}
	return
}

func (s *streamPool) SendAsync(peers []string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	s.lastUsage.Store(time.Now().Unix())
	getStreams := func() (streams []spacesyncproto.SpaceStream) {
		for _, pId := range peers {
			stream, err := s.getOrDeleteStream(pId)
			if err != nil {
				continue
			}
			streams = append(streams, stream)
		}
		return streams
	}

	s.Lock()
	streams := getStreams()
	s.Unlock()

	log.With("objectId", message.ObjectId).
		Debugf("sending message to %d peers", len(streams))
	for _, s := range streams {
		err = s.Send(message)
	}
	if len(peers) != 1 {
		err = nil
	}
	return err
}

func (s *streamPool) getOrDeleteStream(id string) (stream spacesyncproto.SpaceStream, err error) {
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
			break
		}
		streams = append(streams, stream)
	}

	return
}

func (s *streamPool) BroadcastAsync(message *spacesyncproto.ObjectSyncMessage) (err error) {
	streams := s.getAllStreams()
	log.With("objectId", message.ObjectId).
		Debugf("broadcasting message to %d peers", len(streams))
	for _, stream := range streams {
		if err = stream.Send(message); err != nil {
			// TODO: add logging
		}
	}

	return nil
}

func (s *streamPool) AddAndReadStreamAsync(stream spacesyncproto.SpaceStream) {
	go s.AddAndReadStreamSync(stream)
}

func (s *streamPool) AddAndReadStreamSync(stream spacesyncproto.SpaceStream) (err error) {
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
		s.lastUsage.Store(time.Now().Unix())
		if msg.ReplyId == "" {
			s.messageHandler(stream.Context(), peerId, msg)
			return
		}
		log.With("trackingId", msg.ReplyId).Debug("getting message with tracking id")
		s.waitersMx.Lock()
		waiter, exists := s.waiters[msg.ReplyId]

		if !exists {
			log.With("trackingId", msg.ReplyId).Debug("tracking id not exists")
			s.waitersMx.Unlock()
			s.messageHandler(stream.Context(), peerId, msg)
			return
		}
		log.With("trackingId", msg.ReplyId).Debug("tracking id exists")

		delete(s.waiters, msg.ReplyId)
		s.waitersMx.Unlock()
		waiter.ch <- msg
	}

Loop:
	for {
		msg, err := stream.Recv()
		s.lastUsage.Store(time.Now().Unix())
		if err != nil {
			break
		}
		select {
		case <-limiter:
		case <-stream.Context().Done():
			break Loop
		}
		go func() {
			process(msg)
			limiter <- struct{}{}
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

func genStreamPoolKey(peerId, treeId string, counter uint64) string {
	return fmt.Sprintf("%s.%s.%d", peerId, treeId, counter)
}
