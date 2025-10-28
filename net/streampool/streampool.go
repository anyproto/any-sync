package streampool

import (
	"fmt"
	"sync"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/net/context"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/streampool/streamhandler"
)

const CName = "common.net.streampool"

var log = logger.NewNamed(CName)

func New() StreamPool {
	return &streamPool{
		streamIdsByPeer: make(map[string][]uint32),
		streamIdsByTag:  make(map[string][]uint32),
		streams:         make(map[uint32]*stream),
		opening:         make(map[string]*openingProcess),
	}
}

type configGetter interface {
	GetStreamConfig() StreamConfig
}

// PeerGetter should dial or return cached peers
type PeerGetter func(ctx context.Context) (peers []peer.Peer, err error)

type MessageQueueId interface {
	MessageQueueId() string
	DrpcMessage() drpc.Message
}

// StreamPool stores opened streams, and it processes them using StreamHandler
// It opens a stream using StreamHandler.OpenStream method, if needed.
// Also, it could send requests to tagged streams or just to specific peers. For example, a stream could be tagged with a specific spaceId.
type StreamPool interface {
	app.ComponentRunnable
	// AddStream adds new outgoing stream into the pool
	AddStream(stream drpc.Stream, queueSize int, tags ...string) (err error)
	// ReadStream adds new incoming stream and synchronously reads it
	ReadStream(stream drpc.Stream, queueSize int, tags ...string) (err error)
	// Send sends a message to given peers. A stream will be opened if it is not cached before. Works async.
	Send(ctx context.Context, msg drpc.Message, target PeerGetter) (err error)
	// SendById sends a message to given peerIds. Works only if stream exists
	SendById(ctx context.Context, msg drpc.Message, peerIds ...string) (err error)
	// Broadcast sends a message to all peers with given tags. Works async.
	Broadcast(ctx context.Context, msg drpc.Message, tags ...string) (err error)
	// AddTagsCtx adds tags to stream, stream will be extracted from ctx
	AddTagsCtx(ctx context.Context, tags ...string) error
	// RemoveTagsCtx removes tags from stream, stream will be extracted from ctx
	RemoveTagsCtx(ctx context.Context, tags ...string) error
	// Streams gets all streams for specific tags
	Streams(tags ...string) (streams []drpc.Stream)
}

type streamPool struct {
	metric          metric.Metric
	handler         streamhandler.StreamHandler
	statService     debugstat.StatService
	streamIdsByPeer map[string][]uint32
	streamIdsByTag  map[string][]uint32
	streams         map[uint32]*stream
	opening         map[string]*openingProcess
	streamConfig    StreamConfig
	dial            *ExecPool
	mu              sync.Mutex
	writeQueueSize  int
	lastStreamId    uint32
}

func (s *streamPool) OutgoingMsg() (count uint32, size uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.streams {
		count += uint32(st.stats.msgCount.Load())
		size += uint64(st.stats.totalSize.Load())
	}
	return
}

func (s *streamPool) Init(a *app.App) (err error) {
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
	s.handler = a.MustComponent(streamhandler.CName).(streamhandler.StreamHandler)
	comp, ok := a.Component(debugstat.CName).(debugstat.StatService)
	if !ok {
		comp = debugstat.NewNoOp()
	}
	s.statService = comp
	s.streamConfig = a.MustComponent("config").(configGetter).GetStreamConfig()
	s.statService.AddProvider(s)
	if s.metric != nil {
		s.metric.RegisterStreamPoolSyncMetric(s)
		registerMetrics(s.metric.Registry(), s, "")
	}
	return nil
}

func (s *streamPool) Name() (name string) {
	return CName
}

func (s *streamPool) Run(ctx context.Context) (err error) {
	s.dial = NewExecPool(s.streamConfig.DialQueueWorkers, s.streamConfig.DialQueueSize)
	s.dial.Run()
	return nil
}

func (s *streamPool) ProvideStat() any {
	s.mu.Lock()
	var totalSize int64
	var stats []streamStat
	for _, st := range s.streams {
		cp := st.stats
		cp.TotalSize = cp.totalSize.Load()
		cp.MsgCount = int(cp.msgCount.Load())
		totalSize += cp.TotalSize
		stats = append(stats, cp)
	}
	s.mu.Unlock()
	slices.SortFunc(stats, func(first streamStat, second streamStat) int {
		if first.TotalSize > second.TotalSize {
			return -1
		} else if first.TotalSize == second.TotalSize {
			return 0
		} else {
			return 1
		}
	})
	return streamPoolStat{
		TotalSize: totalSize,
		Streams:   stats,
	}
}

func (s *streamPool) StatId() string {
	return CName
}

func (s *streamPool) StatType() string {
	return CName
}

type openingProcess struct {
	ch  chan struct{}
	err error
}

func (s *streamPool) ReadStream(drpcStream drpc.Stream, queueSize int, tags ...string) error {
	st, err := s.addStream(drpcStream, queueSize, tags...)
	if err != nil {
		return err
	}
	go func() {
		st.writeLoop()
	}()
	return st.readLoop()
}

func (s *streamPool) AddStream(drpcStream drpc.Stream, queueSize int, tags ...string) error {
	st, err := s.addStream(drpcStream, queueSize, tags...)
	if err != nil {
		return err
	}
	go func() {
		_ = st.readLoop()
	}()
	go func() {
		st.writeLoop()
	}()
	return nil
}

func (s *streamPool) Streams(tags ...string) (streams []drpc.Stream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tag := range tags {
		for _, id := range s.streamIdsByTag[tag] {
			streams = append(streams, s.streams[id].stream)
		}
	}
	return
}

func (s *streamPool) addStream(drpcStream drpc.Stream, queueSize int, tags ...string) (*stream, error) {
	ctx := drpcStream.Context()
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastStreamId++
	streamId := s.lastStreamId
	if queueSize <= 0 {
		queueSize = 100
	}
	st := &stream{
		peerId:   peerId,
		peerCtx:  ctx,
		stream:   drpcStream,
		pool:     s,
		streamId: streamId,
		l:        log.With(zap.String("peerId", peerId), zap.Uint32("streamId", streamId)),
		tags:     tags,
		stats:    newStreamStat(peerId),
	}
	st.queue = mb.New[drpc.Message](queueSize)
	s.streams[streamId] = st
	s.streamIdsByPeer[peerId] = append(s.streamIdsByPeer[peerId], streamId)
	for _, tag := range tags {
		s.streamIdsByTag[tag] = append(s.streamIdsByTag[tag], streamId)
	}
	return st, nil
}

func (s *streamPool) Send(ctx context.Context, msg drpc.Message, peerGetter PeerGetter) (err error) {
	return s.dial.TryAdd(func() {
		peers, dialErr := peerGetter(ctx)
		if dialErr != nil {
			log.InfoCtx(ctx, "can't get peers", zap.Error(dialErr))
		}
		for _, p := range peers {
			if e := s.sendOne(ctx, p, msg); e != nil {
				log.InfoCtx(ctx, "send peer error", zap.Error(e), zap.String("peerId", p.Id()))
			}
		}
	})
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

	for _, streams := range streamsByPeer {
		for _, st := range streams {
			if e := st.write(msg); e != nil {
				st.l.Debug("sendById write error", zap.Error(e))
			} else {
				return
			}
		}
	}
	if len(streamsByPeer) == 0 {
		return net.ErrUnableToConnect
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
		// in case there was no peerId in context
		ctx := peer.CtxWithPeerId(ctx, p.Id())
		peerProto, err := peer.CtxProtoVersion(p.Context())
		if err != nil {
			peerProto = secureservice.ProtoVersion
		}
		ctx = peer.CtxWithProtoVersion(ctx, peerProto)
		// open new stream and add to pool
		st, tags, queueSize, err := s.handler.OpenStream(ctx, p)
		if err != nil {
			op.err = err
			return
		}
		if err = s.AddStream(st, queueSize, tags...); err != nil {
			op.err = nil
			return
		}
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
	for _, st := range streams {
		if e := st.write(msg); e != nil {
			st.l.InfoCtx(ctx, "broadcast write error", zap.Error(e))
		} else {
			st.l.DebugCtx(ctx, "broadcast success")
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

func (s *streamPool) Close(ctx context.Context) (err error) {
	s.statService.RemoveProvider(s)
	if s.metric != nil {
		s.metric.UnregisterStreamPoolSyncMetric()
	}
	return s.dial.Close()
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
