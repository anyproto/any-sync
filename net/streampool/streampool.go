package streampool

import (
	"fmt"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/pool"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/net/context"
	"storj.io/drpc"
	"sync"
)

// StreamHandler handles incoming messages from streams
type StreamHandler interface {
	// OpenStream opens stream with given peer
	OpenStream(ctx context.Context, p peer.Peer) (stream drpc.Stream, tags []string, err error)
	// HandleMessage handles incoming message
	HandleMessage(ctx context.Context, peerId string, msg drpc.Message) (err error)
	// NewReadMessage creates new empty message for unmarshalling into it
	NewReadMessage() drpc.Message
}

// StreamPool keeps and read streams
type StreamPool interface {
	// AddStream adds new outgoing stream into the pool
	AddStream(peerId string, stream drpc.Stream, tags ...string)
	// ReadStream adds new incoming stream and synchronously read it
	ReadStream(peerId string, stream drpc.Stream, tags ...string) (err error)
	// Send sends a message to given peers. A stream will be opened if it is not cached before. Works async.
	Send(ctx context.Context, msg drpc.Message, peers ...peer.Peer) (err error)
	// SendById sends a message to given peerIds. Works only if stream exists
	SendById(ctx context.Context, msg drpc.Message, peerIds ...string) (err error)
	// Broadcast sends a message to all peers with given tags. Works async.
	Broadcast(ctx context.Context, msg drpc.Message, tags ...string) (err error)
	// AddTagsCtx adds tags to stream, stream will be extracted from ctx
	AddTagsCtx(ctx context.Context, tags ...string) error
	// RemoveTagsCtx removes tags from stream, stream will be extracted from ctx
	RemoveTagsCtx(ctx context.Context, tags ...string) error
	// Close closes all streams
	Close() error
}

type streamPool struct {
	handler         StreamHandler
	streamIdsByPeer map[string][]uint32
	streamIdsByTag  map[string][]uint32
	streams         map[uint32]*stream
	opening         map[string]*openingProcess
	exec            *sendPool
	mu              sync.RWMutex
	lastStreamId    uint32
}

type openingProcess struct {
	ch  chan struct{}
	err error
}
type handleMessage struct {
	ctx    context.Context
	msg    drpc.Message
	peerId string
}

func (s *streamPool) ReadStream(peerId string, drpcStream drpc.Stream, tags ...string) error {
	st := s.addStream(peerId, drpcStream, tags...)
	return st.readLoop()
}

func (s *streamPool) AddStream(peerId string, drpcStream drpc.Stream, tags ...string) {
	st := s.addStream(peerId, drpcStream, tags...)
	go func() {
		_ = st.readLoop()
	}()
}

func (s *streamPool) addStream(peerId string, drpcStream drpc.Stream, tags ...string) *stream {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastStreamId++
	streamId := s.lastStreamId
	st := &stream{
		peerId:   peerId,
		stream:   drpcStream,
		pool:     s,
		streamId: streamId,
		l:        log.With(zap.String("peerId", peerId), zap.Uint32("streamId", streamId)),
		tags:     tags,
	}
	s.streams[streamId] = st
	s.streamIdsByPeer[peerId] = append(s.streamIdsByPeer[peerId], streamId)
	for _, tag := range tags {
		s.streamIdsByTag[tag] = append(s.streamIdsByTag[tag], streamId)
	}
	return st
}

func (s *streamPool) Send(ctx context.Context, msg drpc.Message, peers ...peer.Peer) (err error) {
	var sendOneFunc = func(sp peer.Peer) func() {
		return func() {
			if e := s.sendOne(ctx, sp, msg); e != nil {
				log.InfoCtx(ctx, "send peer error", zap.Error(e), zap.String("peerId", sp.Id()))
			} else {
				log.DebugCtx(ctx, "send success", zap.String("peerId", sp.Id()))
			}
		}
	}

	for _, p := range peers {
		if err = s.exec.Add(ctx, sendOneFunc(p)); err != nil {
			return
		}
	}
	return
}

func (s *streamPool) SendById(ctx context.Context, msg drpc.Message, peerIds ...string) (err error) {
	s.mu.Lock()
	var streamsByPeer [][]*stream
	for _, peerId := range peerIds {
		var streams []*stream
		for _, streamId := range s.streamIdsByPeer[peerId] {
			streams = append(streams, s.streams[streamId])
		}
		if len(streams) != 0 {
			streamsByPeer = append(streamsByPeer, streams)
		}
	}
	s.mu.Unlock()

	var sendStreamsFunc = func(streams []*stream) func() {
		return func() {
			for _, st := range streams {
				if e := st.write(msg); e != nil {
					st.l.Debug("sendById write error", zap.Error(e))
				} else {
					st.l.DebugCtx(ctx, "sendById success")
					return
				}
			}
		}
	}

	for _, streams := range streamsByPeer {
		if err = s.exec.Add(ctx, sendStreamsFunc(streams)); err != nil {
			return
		}
	}
	if len(streamsByPeer) == 0 {
		return pool.ErrUnableToConnect
	}
	return
}

func (s *streamPool) sendOne(ctx context.Context, p peer.Peer, msg drpc.Message) (err error) {
	// get all streams relates to peer
	streams, err := s.getStreams(ctx, p)
	if err != nil {
		return
	}
	for _, st := range streams {
		if err = st.write(msg); err != nil {
			st.l.InfoCtx(ctx, "sendOne write error", zap.Error(err), zap.Int("streams", len(streams)))
			// continue with next stream
			continue
		} else {
			st.l.DebugCtx(ctx, "sendOne success")
			// stop sending on success
			break
		}
	}
	return
}

func (s *streamPool) getStreams(ctx context.Context, p peer.Peer) (streams []*stream, err error) {
	s.mu.Lock()
	// check cached streams
	streamIds := s.streamIdsByPeer[p.Id()]
	for _, streamId := range streamIds {
		streams = append(streams, s.streams[streamId])
	}
	var op *openingProcess
	// no cached streams found
	if len(streams) == 0 {
		// start opening process
		op = s.openStream(ctx, p)
	}
	s.mu.Unlock()

	// not empty openingCh means we should wait for the stream opening and try again
	if op != nil {
		select {
		case <-op.ch:
			if op.err != nil {
				return nil, op.err
			}
			return s.getStreams(ctx, p)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return streams, nil
}

func (s *streamPool) openStream(ctx context.Context, p peer.Peer) *openingProcess {
	if op, ok := s.opening[p.Id()]; ok {
		// already have an opening process for this stream - return channel
		return op
	}
	op := &openingProcess{
		ch: make(chan struct{}),
	}
	s.opening[p.Id()] = op
	go func() {
		// start stream opening in separate goroutine to avoid lock whole pool
		defer func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			close(op.ch)
			delete(s.opening, p.Id())
		}()
		// open new stream and add to pool
		st, tags, err := s.handler.OpenStream(ctx, p)
		if err != nil {
			op.err = err
			return
		}
		s.AddStream(p.Id(), st, tags...)
	}()
	return op
}

func (s *streamPool) Broadcast(ctx context.Context, msg drpc.Message, tags ...string) (err error) {
	s.mu.Lock()
	var streams []*stream
	for _, tag := range tags {
		for _, streamId := range s.streamIdsByTag[tag] {
			streams = append(streams, s.streams[streamId])
		}
	}
	s.mu.Unlock()
	var sendStreamFunc = func(st *stream) func() {
		return func() {
			if e := st.write(msg); e != nil {
				st.l.InfoCtx(ctx, "broadcast write error", zap.Error(e))
			} else {
				st.l.DebugCtx(ctx, "broadcast success")
			}
		}
	}
	for _, st := range streams {
		if st == nil {
			panic("nil stream")
		}
		if err = s.exec.Add(ctx, sendStreamFunc(st)); err != nil {
			return err
		}
	}
	return
}

func (s *streamPool) AddTagsCtx(ctx context.Context, tags ...string) error {
	streamId, ok := ctx.Value(streamCtxKeyStreamId).(uint32)
	if !ok {
		return fmt.Errorf("context without streamId")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.streams[streamId]
	if !ok {
		return fmt.Errorf("stream not found")
	}
	var newTags = make([]string, 0, len(tags))
	for _, newTag := range tags {
		if !slices.Contains(st.tags, newTag) {
			st.tags = append(st.tags, newTag)
			newTags = append(newTags, newTag)
		}
	}
	for _, newTag := range newTags {
		s.streamIdsByTag[newTag] = append(s.streamIdsByTag[newTag], streamId)
	}
	return nil
}

func (s *streamPool) RemoveTagsCtx(ctx context.Context, tags ...string) error {
	streamId, ok := ctx.Value(streamCtxKeyStreamId).(uint32)
	if !ok {
		return fmt.Errorf("context without streamId")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.streams[streamId]
	if !ok {
		return fmt.Errorf("stream not found")
	}

	var filtered = st.tags[:0]
	var toRemove = make([]string, 0, len(tags))
	for _, t := range st.tags {
		if slices.Contains(tags, t) {
			toRemove = append(toRemove, t)
		} else {
			filtered = append(filtered, t)
		}
	}
	st.tags = filtered
	for _, t := range toRemove {
		removeStream(s.streamIdsByTag, t, streamId)
	}
	return nil
}

func (s *streamPool) removeStream(streamId uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.streams[streamId]
	if st == nil {
		log.Fatal("removeStream: stream does not exist", zap.Uint32("streamId", streamId))
	}

	removeStream(s.streamIdsByPeer, st.peerId, streamId)
	for _, tag := range st.tags {
		removeStream(s.streamIdsByTag, tag, streamId)
	}

	delete(s.streams, streamId)
	st.l.Debug("stream removed", zap.Strings("tags", st.tags))
}

func (s *streamPool) Close() (err error) {
	return s.exec.Close()
}

func removeStream(m map[string][]uint32, key string, streamId uint32) {
	streamIds := m[key]
	idx := slices.Index(streamIds, streamId)
	if idx == -1 {
		log.Fatal("removeStream: streamId does not exist", zap.Uint32("streamId", streamId))
	}
	streamIds = slices.Delete(streamIds, idx, idx+1)
	if len(streamIds) == 0 {
		delete(m, key)
	} else {
		m[key] = streamIds
	}
}
