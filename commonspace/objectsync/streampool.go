package objectsync

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/any-sync/app/ocache"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/net/peer"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"
)

var ErrEmptyPeer = errors.New("don't have such a peer")
var ErrStreamClosed = errors.New("stream is already closed")

var maxStreamReaders = 10
var syncWaitPeriod = 2 * time.Second

var ErrSyncTimeout = errors.New("too long wait on sync receive")

// StreamPool can be made generic to work with different streams
type StreamPool interface {
	ocache.ObjectLastUsage
	AddAndReadStreamSync(stream spacesyncproto.ObjectSyncStream) (err error)
	AddAndReadStreamAsync(stream spacesyncproto.ObjectSyncStream) (err error)

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
	peerStreams    map[string]spacesyncproto.ObjectSyncStream
	messageHandler MessageHandler
	wg             *sync.WaitGroup
	waiters        map[string]responseWaiter
	waitersMx      sync.Mutex
	counter        atomic.Uint64
	lastUsage      atomic.Int64
}

func newStreamPool(messageHandler MessageHandler) StreamPool {
	s := &streamPool{
		peerStreams:    make(map[string]spacesyncproto.ObjectSyncStream),
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

		log.With(zap.String("replyId", msg.ReplyId)).Error("time elapsed when waiting")
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
	getStreams := func() (streams []spacesyncproto.ObjectSyncStream) {
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

	log.With(zap.String("objectId", message.ObjectId), zap.Int("existing peers len", len(streams)), zap.Strings("wanted peers", peers)).
		Debug("sending message to peers")
	for _, stream := range streams {
		err = stream.Send(message)
		if err != nil {
			log.Debug("error sending message to stream", zap.Error(err))
		}
	}
	if len(peers) != 1 {
		err = nil
	}
	return err
}

func (s *streamPool) getOrDeleteStream(id string) (stream spacesyncproto.ObjectSyncStream, err error) {
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

func (s *streamPool) getAllStreams() (streams []spacesyncproto.ObjectSyncStream) {
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
		log.With(zap.String("id", id)).Debug("getting peer stream")
		streams = append(streams, stream)
	}

	return
}

func (s *streamPool) BroadcastAsync(message *spacesyncproto.ObjectSyncMessage) (err error) {
	streams := s.getAllStreams()
	log.With(zap.String("objectId", message.ObjectId), zap.Int("peers", len(streams))).
		Debug("broadcasting message to peers")
	for _, stream := range streams {
		if err = stream.Send(message); err != nil {
			log.Debug("error sending message to stream", zap.Error(err))
		}
	}

	return nil
}

func (s *streamPool) AddAndReadStreamAsync(stream spacesyncproto.ObjectSyncStream) (err error) {
	peerId, err := s.addStream(stream)
	if err != nil {
		return
	}
	go s.readPeerLoop(peerId, stream)
	return
}

func (s *streamPool) AddAndReadStreamSync(stream spacesyncproto.ObjectSyncStream) (err error) {
	peerId, err := s.addStream(stream)
	if err != nil {
		return
	}
	return s.readPeerLoop(peerId, stream)
}

func (s *streamPool) addStream(stream spacesyncproto.ObjectSyncStream) (peerId string, err error) {
	s.Lock()
	peerId, err = peer.CtxPeerId(stream.Context())
	if err != nil {
		s.Unlock()
		return
	}
	log.With(zap.String("peer id", peerId)).Debug("adding stream")

	if oldStream, ok := s.peerStreams[peerId]; ok {
		s.Unlock()
		oldStream.Close()
		s.Lock()
		log.With(zap.String("peer id", peerId)).Debug("closed old stream before adding")
	}

	s.peerStreams[peerId] = stream
	s.wg.Add(1)
	s.Unlock()
	return
}

func (s *streamPool) Close() (err error) {
	s.Lock()
	wg := s.wg
	s.Unlock()
	streams := s.getAllStreams()

	log.Debug("closing streams on lock")
	for _, stream := range streams {
		stream.Close()
	}
	log.Debug("closed streams")

	if wg != nil {
		wg.Wait()
	}
	return nil
}

func (s *streamPool) readPeerLoop(peerId string, stream spacesyncproto.ObjectSyncStream) (err error) {
	var (
		log   = log.With(zap.String("peerId", peerId))
		queue = NewDefaultActionQueue()
	)
	queue.Run()

	defer func() {
		log.Debug("stopped reading stream from peer")
		s.removePeer(peerId, stream)
		queue.Close()
		s.wg.Done()
	}()

	log.Debug("started reading stream from peer")

	stopWaiter := func(msg *spacesyncproto.ObjectSyncMessage) bool {
		s.waitersMx.Lock()
		waiter, exists := s.waiters[msg.ReplyId]
		if exists {
			delete(s.waiters, msg.ReplyId)
			s.waitersMx.Unlock()
			waiter.ch <- msg
			return true
		}
		s.waitersMx.Unlock()
		return false
	}

	process := func(msg *spacesyncproto.ObjectSyncMessage) error {
		log := log.With(zap.String("replyId", msg.ReplyId), zap.String("object id", msg.ObjectId))
		log.Debug("getting message with reply id")
		err = s.messageHandler(stream.Context(), peerId, msg)
		if err != nil {
			log.With(zap.Error(err)).Debug("message handling failed")
		}
		return nil
	}

	for {
		select {
		case <-stream.Context().Done():
			return
		default:
		}
		var msg *spacesyncproto.ObjectSyncMessage
		msg, err = stream.Recv()
		s.lastUsage.Store(time.Now().Unix())
		if err != nil {
			stream.Close()
			return
		}

		if msg.ReplyId != "" {
			// then we can send it directly to waiters without adding to queue or starting a reader
			if stopWaiter(msg) {
				continue
			}
			log.With(zap.String("replyId", msg.ReplyId)).Debug("reply id does not exist")
		}

		queue.Send(func() error {
			return process(msg)
		})
	}
}

func (s *streamPool) removePeer(peerId string, stream spacesyncproto.ObjectSyncStream) (err error) {
	s.Lock()
	defer s.Unlock()
	mapStream, ok := s.peerStreams[peerId]
	if !ok {
		return ErrEmptyPeer
	}

	// it can be the case that the stream was already replaced
	if mapStream == stream {
		delete(s.peerStreams, peerId)
	}
	return
}

func genStreamPoolKey(peerId, treeId string, counter uint64) string {
	return fmt.Sprintf("%s.%s.%d", peerId, treeId, counter)
}
