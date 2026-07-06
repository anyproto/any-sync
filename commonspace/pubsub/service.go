package pubsub

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/crypto"
)

const CName = "common.commonspace.pubsub"

var log = logger.NewNamed(CName)

var (
	ErrClosed = errors.New("pubsub service closed")
)

// Service is the shared pubsub engine used by nodes (relay role, Deps.Relay set)
// and clients (Deps.Peers set) alike. One long-lived PubSubStream per peer pair
// multiplexes all spaces and topics; interest and routing state are in-memory only
// and die with the streams.
type Service interface {
	app.ComponentRunnable
	// Publish encrypts, signs and fire-and-forgets payload to the topic within the space.
	Publish(ctx context.Context, spaceId, topic string, payload []byte) error
	// Subscribe registers a local handler for a pattern and pushes the interest to
	// the space's peers. The returned func unregisters and unsubscribes.
	Subscribe(spaceId, pattern string, h Handler) (unsubscribe func(), err error)
	// SyncInterest (re)sends all local interest for the space to its current peers;
	// call after (re)connecting to a space's peers.
	SyncInterest(ctx context.Context, spaceId string) error
	// CloseSpace drops all local and serving-side interest for the space and
	// withdraws the local interest from its peers. Hosts call it on space
	// unload/close so a global service doesn't retain closed-space state.
	CloseSpace(spaceId string)
	// EvictMember drops all serving-side interest of an identity in a space,
	// enforcing DESIGN §6.4 (active drop on ACL removal). Nodes wire it to their
	// ACL-update hook. No-op on clients (they hold no serving interest).
	EvictMember(spaceId string, identity crypto.PubKey)
	// RevalidateMembers evicts every subscriber of the space whose account no
	// longer passes isMember. Nodes call it on an ACL change to cut off all
	// removed members in one pass. No-op on clients.
	RevalidateMembers(spaceId string, isMember func(account string) bool)
	// HandleStream serves an inbound PubSubStream; blocks for the stream lifetime.
	HandleStream(stream drpc.Stream) error
}

func New(deps Deps) Service {
	return &service{deps: deps}
}

type service struct {
	deps    Deps
	cfg     Config
	account *accountdata.AccountKeys
	pool    streampool.StreamPool

	// remote interest: what peers subscribed on us (serving side).
	// Interest is keyed by streamId (not peerId): two streams from the same peer
	// are independent, so a reconnecting peer's fresh stream keeps its interest
	// when the stale stream closes, and the trie refcount counts subscribing
	// streams. streams[streamId] is the authoritative record cleaned on close.
	remoteMu sync.Mutex
	remote   map[string]*spaceInterest  // spaceId -> match trie (refcount = subscribing streams)
	streams  map[uint32]*streamInterest // streamId -> what that stream subscribed

	// local interest: what our handlers subscribed to (client side)
	localMu    sync.Mutex
	localTrie  map[string]*patternTrie          // spaceId -> trie of local patterns
	localSubs  map[string]map[string][]localSub // spaceId -> pattern -> handlers
	nextSubId  uint64
	localTopic map[string]int // spaceId -> distinct local patterns (cap bookkeeping)

	dedup     *msgIdDedup
	rate      *peerRateLimiter
	dispatch  *mb.MB[dispatchItem]
	resyncNow chan struct{}
	loops     sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

type spaceInterest struct {
	trie *patternTrie
}

// streamInterest records the patterns one inbound stream subscribed, per space,
// so a stream close can withdraw exactly its own contribution regardless of what
// tags the pool recorded.
type streamInterest struct {
	peerId  string
	account string                         // subscriber account id, for member eviction
	bySpace map[string]map[string]struct{} // spaceId -> patterns
	total   int                            // total patterns across spaces on this stream
}

type localSub struct {
	id uint64
	h  Handler
}

type dispatchItem struct {
	patterns []string
	spaceId  string
	topic    string
	identity crypto.PubKey
	payload  []byte
}

func (s *service) Init(a *app.App) (err error) {
	s.cfg = s.deps.Config.withDefaults()
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service).Account()
	poolOpts := []streampool.Option{streampool.WithStreamCloseHook(s.onStreamClose)}
	if s.deps.Metric != nil {
		poolOpts = append(poolOpts, streampool.WithMetric(s.deps.Metric, "pubsub"))
	}
	s.pool = streampool.NewStreamPool(s, s.cfg.streamPoolConfig(), poolOpts...)
	s.remote = make(map[string]*spaceInterest)
	s.streams = make(map[uint32]*streamInterest)
	s.localTrie = make(map[string]*patternTrie)
	s.localSubs = make(map[string]map[string][]localSub)
	s.localTopic = make(map[string]int)
	s.dedup = newMsgIdDedup(s.cfg.DedupSize)
	s.rate = newPeerRateLimiter(s.cfg.PublishRps, s.cfg.PublishBurst)
	s.dispatch = mb.New[dispatchItem](s.cfg.DispatchQueueSize)
	s.resyncNow = make(chan struct{}, 1)
	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	return nil
}

func (s *service) Name() string { return CName }

func (s *service) Run(ctx context.Context) error {
	if err := s.pool.Run(ctx); err != nil {
		return err
	}
	s.loops.Add(1)
	go func() {
		defer s.loops.Done()
		s.dispatchLoop()
	}()
	if s.deps.Peers != nil {
		s.loops.Add(1)
		go func() {
			defer s.loops.Done()
			s.resyncLoop()
		}()
	}
	return nil
}

// resyncLoop keeps server-side interest alive across reconnects. streampool opens
// streams lazily and never re-sends interest on a fresh stream, so without this a
// subscriber goes silent after its stream drops (a routine event: idle pool GC
// reaps quiet streams). The loop re-pushes all local interest periodically, and
// immediately when a client-side stream closes. Re-sends are idempotent
// (handleSubscribe dedups per stream) and bounded by the local subscription set.
func (s *service) resyncLoop() {
	ticker := time.NewTicker(s.cfg.ResyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		case <-s.resyncNow:
		}
		s.localMu.Lock()
		spaces := make([]string, 0, len(s.localSubs))
		for spaceId := range s.localSubs {
			spaces = append(spaces, spaceId)
		}
		s.localMu.Unlock()
		for _, spaceId := range spaces {
			if err := s.SyncInterest(s.ctx, spaceId); err != nil {
				log.Debug("resync interest failed", zap.String("spaceId", spaceId), zap.Error(err))
			}
		}
	}
}

// triggerResync asks the resync loop to re-push interest now (non-blocking).
func (s *service) triggerResync() {
	select {
	case s.resyncNow <- struct{}{}:
	default:
	}
}

func (s *service) Close(ctx context.Context) error {
	s.ctxCancel()
	_ = s.dispatch.Close()
	s.loops.Wait()
	return s.pool.Close(ctx)
}

//
// public API (client side)
//

func (s *service) Publish(ctx context.Context, spaceId, topic string, payload []byte) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}
	if len(payload) > s.cfg.MaxPayloadSize {
		return pubsubproto.ErrInvalidMessage
	}
	if owner := TopicOwner(topic); owner != "" && owner != s.account.SignKey.GetPublic().Account() {
		return pubsubproto.ErrTopicNotOwned
	}
	p := &pubsubproto.Publish{
		SpaceId:        spaceId,
		Topic:          topic,
		MsgId:          make([]byte, msgIdLen),
		TimestampMilli: time.Now().UnixMilli(),
	}
	if _, err := rand.Read(p.MsgId); err != nil {
		return err
	}
	if s.deps.Crypto != nil {
		keyId, enc, err := s.deps.Crypto.Encrypt(spaceId, payload)
		if err != nil {
			return err
		}
		p.KeyId, p.Payload = keyId, enc
	} else {
		p.Payload = payload
	}
	if err := signPublish(s.account.SignKey, p); err != nil {
		return err
	}
	// record our own msgId so network echoes are suppressed,
	// and deliver to local handlers synchronously with the plaintext
	s.dedup.seen(p.MsgId)
	s.enqueueLocal(spaceId, topic, s.account.SignKey.GetPublic(), payload)

	if s.deps.Peers == nil {
		return nil
	}
	msg := wrapPublish(p)
	return s.pool.Send(ctx, msg, func(ctx context.Context) ([]peer.Peer, error) {
		return s.deps.Peers.SpacePeers(ctx, spaceId)
	})
}

func (s *service) Subscribe(spaceId, pattern string, h Handler) (func(), error) {
	if err := ValidatePattern(pattern); err != nil {
		return nil, err
	}
	s.localMu.Lock()
	if s.localTopic[spaceId] >= s.cfg.MaxPatternsPerSpace {
		s.localMu.Unlock()
		return nil, pubsubproto.ErrTooManyTopics
	}
	trie, ok := s.localTrie[spaceId]
	if !ok {
		trie = newPatternTrie()
		s.localTrie[spaceId] = trie
		s.localSubs[spaceId] = make(map[string][]localSub)
	}
	s.nextSubId++
	id := s.nextSubId
	firstForPattern := len(s.localSubs[spaceId][pattern]) == 0
	s.localSubs[spaceId][pattern] = append(s.localSubs[spaceId][pattern], localSub{id: id, h: h})
	if firstForPattern {
		trie.Add(pattern)
		s.localTopic[spaceId]++
	}
	s.localMu.Unlock()

	if firstForPattern {
		s.sendInterest(spaceId, []string{pattern}, true)
	}
	return func() { s.unsubscribe(spaceId, pattern, id) }, nil
}

func (s *service) unsubscribe(spaceId, pattern string, id uint64) {
	s.localMu.Lock()
	subs := s.localSubs[spaceId][pattern]
	for i, sub := range subs {
		if sub.id == id {
			subs = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	lastForPattern := len(subs) == 0
	if lastForPattern {
		delete(s.localSubs[spaceId], pattern)
		if trie := s.localTrie[spaceId]; trie != nil {
			trie.Remove(pattern)
			s.localTopic[spaceId]--
			if trie.Len() == 0 {
				delete(s.localTrie, spaceId)
				delete(s.localSubs, spaceId)
				delete(s.localTopic, spaceId)
			}
		}
	} else {
		s.localSubs[spaceId][pattern] = subs
	}
	s.localMu.Unlock()

	if lastForPattern {
		s.sendInterest(spaceId, []string{pattern}, false)
	}
}

func (s *service) SyncInterest(ctx context.Context, spaceId string) error {
	s.localMu.Lock()
	var patterns []string
	for pattern := range s.localSubs[spaceId] {
		patterns = append(patterns, pattern)
	}
	s.localMu.Unlock()
	if len(patterns) == 0 || s.deps.Peers == nil {
		return nil
	}
	msg := &pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Subscribe{
		Subscribe: &pubsubproto.Subscribe{SpaceId: spaceId, Topics: patterns},
	}}
	return s.pool.Send(ctx, msg, func(ctx context.Context) ([]peer.Peer, error) {
		return s.deps.Peers.SpacePeers(ctx, spaceId)
	})
}

func (s *service) CloseSpace(spaceId string) {
	// client side: drop local subscriptions and withdraw interest from peers
	s.localMu.Lock()
	var patterns []string
	for pattern := range s.localSubs[spaceId] {
		patterns = append(patterns, pattern)
	}
	delete(s.localTrie, spaceId)
	delete(s.localSubs, spaceId)
	delete(s.localTopic, spaceId)
	s.localMu.Unlock()
	if len(patterns) > 0 {
		s.sendInterest(spaceId, patterns, false)
	}

	// serving side: drop the space trie and every stream's interest in it,
	// stripping the routing tags so no lingering tag delivers after close
	s.remoteMu.Lock()
	delete(s.remote, spaceId)
	for streamId, strm := range s.streams {
		spacePatterns := strm.bySpace[spaceId]
		if len(spacePatterns) == 0 {
			continue
		}
		tags := make([]string, 0, len(spacePatterns))
		for pattern := range spacePatterns {
			tags = append(tags, interestTag(spaceId, pattern))
			strm.total--
		}
		delete(strm.bySpace, spaceId)
		_ = s.pool.RemoveTagsById(streamId, tags...)
		if strm.total == 0 {
			delete(s.streams, streamId)
		}
	}
	s.remoteMu.Unlock()
}

func (s *service) EvictMember(spaceId string, identity crypto.PubKey) {
	account := identity.Account()
	s.evictSpaceStreams(spaceId, func(strm *streamInterest) bool {
		return strm.account == account
	})
}

// RevalidateMembers evicts every subscriber of the space whose account no longer
// passes isMember. A node calls it on an ACL change to actively cut off removed
// members (DESIGN §6.4) rather than waiting for their streams to close.
func (s *service) RevalidateMembers(spaceId string, isMember func(account string) bool) {
	s.evictSpaceStreams(spaceId, func(strm *streamInterest) bool {
		return !isMember(strm.account)
	})
}

// evictSpaceStreams drops the space interest of every stream matching evict,
// stripping its routing tags so delivery stops even while the stream stays open.
func (s *service) evictSpaceStreams(spaceId string, evict func(*streamInterest) bool) {
	s.remoteMu.Lock()
	defer s.remoteMu.Unlock()
	si := s.remote[spaceId]
	for streamId, strm := range s.streams {
		spacePatterns := strm.bySpace[spaceId]
		if len(spacePatterns) == 0 || !evict(strm) {
			continue
		}
		tags := make([]string, 0, len(spacePatterns))
		for pattern := range spacePatterns {
			tags = append(tags, interestTag(spaceId, pattern))
			strm.total--
			if si != nil {
				si.trie.Remove(pattern)
			}
		}
		delete(strm.bySpace, spaceId)
		_ = s.pool.RemoveTagsById(streamId, tags...)
		if strm.total == 0 {
			delete(s.streams, streamId)
		}
	}
	if si != nil {
		s.pruneSpace(spaceId, si)
	}
}

func (s *service) sendInterest(spaceId string, patterns []string, subscribe bool) {
	if s.deps.Peers == nil {
		return
	}
	var msg *pubsubproto.PubSubMessage
	if subscribe {
		msg = &pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Subscribe{
			Subscribe: &pubsubproto.Subscribe{SpaceId: spaceId, Topics: patterns},
		}}
	} else {
		msg = &pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Unsubscribe{
			Unsubscribe: &pubsubproto.Unsubscribe{SpaceId: spaceId, Topics: patterns},
		}}
	}
	if err := s.pool.Send(s.ctx, msg, func(ctx context.Context) ([]peer.Peer, error) {
		return s.deps.Peers.SpacePeers(ctx, spaceId)
	}); err != nil {
		log.Info("send interest failed", zap.String("spaceId", spaceId), zap.Error(err))
	}
}

//
// streamhandler.StreamHandler (the engine is its own pool handler)
//

func (s *service) OpenStream(ctx context.Context, p peer.Peer) (stream drpc.Stream, tags []string, queueSize int, err error) {
	// hold the peer so idle pool GC (default ~1m TTL) does not silently reap a
	// quiet pubsub stream out from under a subscriber
	p.SetTTL(s.cfg.PeerTTL)
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	objectStream, err := pubsubproto.NewDRPCPubSubClient(conn).PubSubStream(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	return objectStream, nil, s.cfg.WriteQueueSize, nil
}

func (s *service) NewReadMessage() drpc.Message {
	return &pubsubproto.PubSubMessage{}
}

func (s *service) HandleMessage(ctx context.Context, peerId string, msg drpc.Message) error {
	m, ok := msg.(*pubsubproto.PubSubMessage)
	if !ok {
		return pubsubproto.ErrUnexpected
	}
	switch {
	case m.GetSubscribe() != nil:
		s.handleSubscribe(ctx, peerId, m.GetSubscribe())
	case m.GetUnsubscribe() != nil:
		s.handleUnsubscribe(ctx, peerId, m.GetUnsubscribe())
	case m.GetPublish() != nil:
		s.handlePublish(ctx, peerId, m.GetPublish())
	case m.GetStatus() != nil:
		st := m.GetStatus()
		log.Debug("pubsub status from peer",
			zap.String("peerId", peerId), zap.String("spaceId", st.SpaceId),
			zap.String("code", st.Code.String()), zap.Strings("topics", st.Topics))
		if s.deps.OnStatus != nil {
			s.deps.OnStatus(peerId, st)
		}
	}
	// frame-level rejections answer with Status; never break the stream
	return nil
}

// HandleStream serves an inbound PubSubStream (DRPC server entry); blocks.
func (s *service) HandleStream(stream drpc.Stream) error {
	return s.pool.ReadStream(stream, s.cfg.WriteQueueSize)
}

//
// serving-side frame handling
//

func (s *service) handleSubscribe(ctx context.Context, peerId string, sub *pubsubproto.Subscribe) {
	streamId, ok := streampool.CtxStreamId(ctx)
	if !ok {
		s.sendStatus(ctx, peerId, sub.SpaceId, sub.Topics, pubsubproto.ErrCodes_InvalidMessage)
		return
	}
	identity, err := peer.CtxPubKey(ctx)
	if err != nil {
		s.sendStatus(ctx, peerId, sub.SpaceId, sub.Topics, pubsubproto.ErrCodes_InvalidMessage)
		return
	}
	if err = validateSpaceId(sub.SpaceId); err != nil {
		s.sendStatus(ctx, peerId, sub.SpaceId, sub.Topics, pubsubproto.ErrCodes_InvalidTopic)
		return
	}
	if s.deps.Relay != nil && !s.deps.Relay.IsResponsible(sub.SpaceId) {
		s.sendStatus(ctx, peerId, sub.SpaceId, sub.Topics, pubsubproto.ErrCodes_NotResponsible)
		return
	}
	for _, pattern := range sub.Topics {
		if err = ValidatePattern(pattern); err != nil {
			s.sendStatus(ctx, peerId, sub.SpaceId, sub.Topics, pubsubproto.ErrCodes_InvalidTopic)
			return
		}
	}
	if s.deps.Membership != nil {
		if err = s.deps.Membership.CheckMember(ctx, sub.SpaceId, identity); err != nil {
			s.sendStatus(ctx, peerId, sub.SpaceId, sub.Topics, pubsubproto.ErrCodes_NotAMember)
			return
		}
	}

	// Record interest and register the routing tags under a single remoteMu hold.
	// remoteMu -> pool.mu is a consistent lock order (removeStream releases pool.mu
	// before onStreamClose takes remoteMu), so holding across AddTagsCtx is safe and
	// makes interest+tag atomic w.r.t. a concurrent close: if the stream was already
	// removed, AddTagsCtx fails and we roll the interest back, so nothing leaks.
	s.remoteMu.Lock()
	si := s.remote[sub.SpaceId]
	if si == nil {
		si = &spaceInterest{trie: newPatternTrie()}
		s.remote[sub.SpaceId] = si
	}
	strm := s.streams[streamId]
	if strm == nil {
		strm = &streamInterest{peerId: peerId, account: identity.Account(), bySpace: make(map[string]map[string]struct{})}
		s.streams[streamId] = strm
	}
	spacePatterns := strm.bySpace[sub.SpaceId]
	if spacePatterns == nil {
		spacePatterns = make(map[string]struct{})
		strm.bySpace[sub.SpaceId] = spacePatterns
	}
	var accepted []string
	var capExceeded bool
	for _, pattern := range sub.Topics {
		if _, exists := spacePatterns[pattern]; exists {
			continue
		}
		if len(spacePatterns) >= s.cfg.MaxPatternsPerSpace || strm.total >= s.cfg.MaxPatternsPerStream {
			capExceeded = true
			break
		}
		spacePatterns[pattern] = struct{}{}
		strm.total++
		si.trie.Add(pattern)
		accepted = append(accepted, pattern)
	}
	if len(accepted) > 0 {
		tags := make([]string, len(accepted))
		for i, pattern := range accepted {
			tags[i] = interestTag(sub.SpaceId, pattern)
		}
		if err = s.pool.AddTagsCtx(ctx, tags...); err != nil {
			// the stream was removed between accept and tagging: undo the interest
			// so onStreamClose (which found nothing) leaves no orphan behind
			for _, pattern := range accepted {
				s.removeStreamPattern(strm, si, sub.SpaceId, pattern)
			}
			s.pruneStream(streamId, strm)
			s.pruneSpace(sub.SpaceId, si)
		}
	}
	s.remoteMu.Unlock()

	if capExceeded {
		s.sendStatus(ctx, peerId, sub.SpaceId, sub.Topics, pubsubproto.ErrCodes_TooManyTopics)
	}
}

func (s *service) handleUnsubscribe(ctx context.Context, peerId string, unsub *pubsubproto.Unsubscribe) {
	streamId, ok := streampool.CtxStreamId(ctx)
	if !ok {
		return
	}
	s.remoteMu.Lock()
	strm := s.streams[streamId]
	si := s.remote[unsub.SpaceId]
	if strm == nil || si == nil {
		s.remoteMu.Unlock()
		return
	}
	patterns := unsub.Topics
	if len(patterns) == 0 { // empty means all patterns of the space
		for pattern := range strm.bySpace[unsub.SpaceId] {
			patterns = append(patterns, pattern)
		}
	}
	var removed []string
	for _, pattern := range patterns {
		if s.removeStreamPattern(strm, si, unsub.SpaceId, pattern) {
			removed = append(removed, pattern)
		}
	}
	s.pruneStream(streamId, strm)
	s.pruneSpace(unsub.SpaceId, si)
	s.remoteMu.Unlock()

	if len(removed) > 0 {
		tags := make([]string, len(removed))
		for i, pattern := range removed {
			tags[i] = interestTag(unsub.SpaceId, pattern)
		}
		if err := s.pool.RemoveTagsCtx(ctx, tags...); err != nil {
			log.Warn("remove tags failed", zap.Error(err))
		}
	}
}

func (s *service) handlePublish(ctx context.Context, peerId string, p *pubsubproto.Publish) {
	if len(p.MsgId) != msgIdLen || len(p.Payload) > s.cfg.MaxPayloadSize {
		s.sendPubStatus(ctx, peerId, p, pubsubproto.ErrCodes_InvalidMessage)
		return
	}
	if ValidateTopic(p.Topic) != nil {
		s.sendPubStatus(ctx, peerId, p, pubsubproto.ErrCodes_InvalidTopic)
		return
	}
	if s.deps.Relay != nil {
		s.relayPublish(ctx, peerId, p)
		return
	}
	// client role: deliver locally only, never forward (relay rule 1)
	s.receivePublish(ctx, p)
}

// relayPublish is the node ingress path: authorize, fan out to subscribed streams,
// and forward client-originated messages once to the other responsible nodes.
func (s *service) relayPublish(ctx context.Context, peerId string, p *pubsubproto.Publish) {
	if !s.deps.Relay.IsResponsible(p.SpaceId) {
		s.sendPubStatus(ctx, peerId, p, pubsubproto.ErrCodes_NotResponsible)
		return
	}
	if p.Relayed {
		// only responsible peer nodes may relay; relayed messages are never re-forwarded
		if !s.deps.Relay.IsResponsibleNode(p.SpaceId, peerId) {
			return
		}
		s.fanout(ctx, p)
		return
	}
	// client-originated: bind attribution to the handshake-proven identity.
	// Reject an empty identity explicitly: CtxIdentity returns (nil,nil) for an
	// unverified inbound, and bytesEqual(nil,nil) would otherwise pass the bind.
	ctxIdentity, err := peer.CtxIdentity(ctx)
	if err != nil || len(ctxIdentity) == 0 || len(p.Identity) == 0 || !bytesEqual(ctxIdentity, p.Identity) {
		s.sendPubStatus(ctx, peerId, p, pubsubproto.ErrCodes_InvalidMessage)
		return
	}
	if s.deps.Membership != nil {
		identity, err := peer.CtxPubKey(ctx)
		if err != nil || s.deps.Membership.CheckMember(ctx, p.SpaceId, identity) != nil {
			s.sendPubStatus(ctx, peerId, p, pubsubproto.ErrCodes_NotAMember)
			return
		}
	}
	if owner := TopicOwner(p.Topic); owner != "" {
		identity, err := peer.CtxPubKey(ctx)
		if err != nil || identity.Account() != owner {
			s.sendPubStatus(ctx, peerId, p, pubsubproto.ErrCodes_TopicNotOwned)
			return
		}
	}
	if !s.rate.allow(peerId) {
		s.sendPubStatus(ctx, peerId, p, pubsubproto.ErrCodes_RateLimited)
		return
	}
	s.fanout(ctx, p)
	// forward exactly once to the other responsible nodes;
	// byte-slice fields are shared with the original but never mutated
	relayed := &pubsubproto.Publish{
		SpaceId:        p.SpaceId,
		Topic:          p.Topic,
		MsgId:          p.MsgId,
		KeyId:          p.KeyId,
		Payload:        p.Payload,
		Identity:       p.Identity,
		Signature:      p.Signature,
		TimestampMilli: p.TimestampMilli,
		Relayed:        true,
	}
	if err := s.pool.Send(ctx, wrapPublish(relayed), func(ctx context.Context) ([]peer.Peer, error) {
		return s.deps.Relay.OtherResponsiblePeers(ctx, p.SpaceId)
	}); err != nil {
		log.Info("forward to responsible nodes failed", zap.Error(err))
	}
}

// fanout writes the message to every stream whose interest matches the topic.
func (s *service) fanout(ctx context.Context, p *pubsubproto.Publish) {
	s.remoteMu.Lock()
	si := s.remote[p.SpaceId]
	var patterns []string
	if si != nil {
		patterns = si.trie.Match(p.Topic, nil)
	}
	s.remoteMu.Unlock()
	if len(patterns) == 0 {
		return
	}
	tags := make([]string, len(patterns))
	for i, pattern := range patterns {
		tags[i] = interestTag(p.SpaceId, pattern)
	}
	// Broadcast dedups streams subscribed to several matching patterns
	if err := s.pool.Broadcast(ctx, wrapPublish(p), tags...); err != nil {
		log.Info("fanout failed", zap.Error(err))
	}
}

// receivePublish is the client receive path. Cheap filters (local interest,
// membership, ownership, timestamp) run before the expensive Ed25519 verify so a
// relay/LAN-peer flood of forged messages is shed cheaply; the dedup ring is
// recorded only after verify so junk msgIds can't evict legitimate ones.
func (s *service) receivePublish(ctx context.Context, p *pubsubproto.Publish) {
	s.localMu.Lock()
	var patterns []string
	if trie := s.localTrie[p.SpaceId]; trie != nil {
		patterns = trie.Match(p.Topic, nil)
	}
	s.localMu.Unlock()
	if len(patterns) == 0 {
		return
	}
	// cheap: unmarshal the claimed identity (not yet trusted) to run filters
	identity, err := identityOf(p)
	if err != nil {
		log.Debug("dropping publish with bad identity", zap.String("topic", p.Topic), zap.Error(err))
		return
	}
	if s.deps.Membership != nil {
		if err = s.deps.Membership.CheckMember(ctx, p.SpaceId, identity); err != nil {
			log.Debug("dropping publish from non-member", zap.String("topic", p.Topic))
			return
		}
	}
	// end-to-end acc/ ownership: the signature covers the topic, so not even a
	// malicious relay can inject into someone else's self-owned topic
	if owner := TopicOwner(p.Topic); owner != "" && identity.Account() != owner {
		log.Debug("dropping publish into unowned topic", zap.String("topic", p.Topic))
		return
	}
	if s.isStale(p.TimestampMilli) {
		log.Debug("dropping stale publish", zap.String("topic", p.Topic))
		return
	}
	// expensive: verify only after the cheap filters pass
	if err = verifySignature(identity, p); err != nil {
		log.Debug("dropping publish with bad signature", zap.String("topic", p.Topic), zap.Error(err))
		return
	}
	// record dedup only for messages we would deliver, so forged floods can't
	// flush the ring and re-open a replay window
	if s.dedup.seen(p.MsgId) {
		return
	}
	payload := p.Payload
	if p.KeyId != "" {
		if s.deps.Crypto == nil {
			return
		}
		if payload, err = s.deps.Crypto.Decrypt(p.SpaceId, p.KeyId, p.Payload); err != nil {
			log.Debug("dropping undecryptable publish", zap.String("topic", p.Topic), zap.Error(err))
			return
		}
	}
	s.enqueueLocalMatched(patterns, p.SpaceId, p.Topic, identity, payload)
}

// isStale reports whether a signed timestamp is outside the accepted skew window,
// raising the replay bar even after dedup eviction. Zero is treated as absent
// (never stale) to stay compatible with senders that omit it.
func (s *service) isStale(timestampMilli int64) bool {
	if timestampMilli == 0 {
		return false
	}
	skew := s.cfg.MaxTimestampSkew.Milliseconds()
	now := time.Now().UnixMilli()
	delta := now - timestampMilli
	return delta > skew || delta < -skew
}

//
// local dispatch
//

func (s *service) enqueueLocal(spaceId, topic string, identity crypto.PubKey, payload []byte) {
	s.localMu.Lock()
	var patterns []string
	if trie := s.localTrie[spaceId]; trie != nil {
		patterns = trie.Match(topic, nil)
	}
	s.localMu.Unlock()
	if len(patterns) == 0 {
		return
	}
	s.enqueueLocalMatched(patterns, spaceId, topic, identity, payload)
}

func (s *service) enqueueLocalMatched(patterns []string, spaceId, topic string, identity crypto.PubKey, payload []byte) {
	err := s.dispatch.TryAdd(dispatchItem{
		patterns: patterns,
		spaceId:  spaceId,
		topic:    topic,
		identity: identity,
		payload:  payload,
	})
	if err != nil && !errors.Is(err, mb.ErrClosed) {
		log.Debug("dispatch queue overflow, message dropped", zap.String("topic", topic))
	}
}

func (s *service) dispatchLoop() {
	for {
		item, err := s.dispatch.WaitOne(s.ctx)
		if err != nil {
			return
		}
		s.localMu.Lock()
		var handlers []Handler
		for _, pattern := range item.patterns {
			for _, sub := range s.localSubs[item.spaceId][pattern] {
				handlers = append(handlers, sub.h)
			}
		}
		s.localMu.Unlock()
		for _, h := range handlers {
			h(item.spaceId, item.topic, item.identity, item.payload)
		}
	}
}

//
// housekeeping
//

// onStreamClose withdraws exactly the closed stream's interest, keyed by streamId.
// It uses the engine's own per-stream record (streams[streamId]) rather than the
// pool's tag snapshot, so a stream that closed before its tags were registered is
// still cleaned, and a sibling stream of the same peer is never touched.
func (s *service) onStreamClose(streamId uint32, _ string, _ []string) {
	// A client-side outbound stream dropping means our pushed interest is gone on
	// the peer; re-push it promptly rather than waiting for the periodic tick.
	if s.deps.Peers != nil {
		s.triggerResync()
	}
	s.remoteMu.Lock()
	defer s.remoteMu.Unlock()
	strm := s.streams[streamId]
	if strm == nil {
		return
	}
	for spaceId, patterns := range strm.bySpace {
		si := s.remote[spaceId]
		if si == nil {
			continue
		}
		for pattern := range patterns {
			si.trie.Remove(pattern)
		}
		s.pruneSpace(spaceId, si)
	}
	delete(s.streams, streamId)
}

// removeStreamPattern withdraws one pattern of one space from a stream's record and
// the space trie. Returns true if the pattern was present. Caller holds remoteMu.
func (s *service) removeStreamPattern(strm *streamInterest, si *spaceInterest, spaceId, pattern string) bool {
	patterns := strm.bySpace[spaceId]
	if _, ok := patterns[pattern]; !ok {
		return false
	}
	delete(patterns, pattern)
	strm.total--
	if len(patterns) == 0 {
		delete(strm.bySpace, spaceId)
	}
	si.trie.Remove(pattern)
	return true
}

// pruneStream drops an empty stream record. Caller holds remoteMu.
func (s *service) pruneStream(streamId uint32, strm *streamInterest) {
	if strm.total == 0 {
		delete(s.streams, streamId)
	}
}

// pruneSpace drops an empty space trie. Caller holds remoteMu.
func (s *service) pruneSpace(spaceId string, si *spaceInterest) {
	if si.trie.Len() == 0 {
		delete(s.remote, spaceId)
	}
}

func (s *service) sendStatus(ctx context.Context, peerId, spaceId string, topics []string, code pubsubproto.ErrCodes) {
	s.sendStatusMsg(ctx, peerId, &pubsubproto.Status{SpaceId: spaceId, Topics: topics, Code: code})
}

// sendPubStatus reports a rejected publish, echoing its msgId so the caller can
// correlate the rejection to the originating Publish.
func (s *service) sendPubStatus(ctx context.Context, peerId string, p *pubsubproto.Publish, code pubsubproto.ErrCodes) {
	s.sendStatusMsg(ctx, peerId, &pubsubproto.Status{
		SpaceId: p.SpaceId,
		Topics:  []string{p.Topic},
		Code:    code,
		MsgId:   p.MsgId,
	})
}

func (s *service) sendStatusMsg(ctx context.Context, peerId string, st *pubsubproto.Status) {
	msg := &pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Status{Status: st}}
	if err := s.pool.SendById(ctx, msg, peerId); err != nil {
		log.Debug("send status failed", zap.String("peerId", peerId), zap.Error(err))
	}
}

func interestTag(spaceId, pattern string) string {
	return spaceId + "/" + pattern
}

func wrapPublish(p *pubsubproto.Publish) *pubsubproto.PubSubMessage {
	return &pubsubproto.PubSubMessage{Content: &pubsubproto.PubSubMessage_Publish{Publish: p}}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
