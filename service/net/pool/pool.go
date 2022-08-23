package pool

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/dialer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
)

const (
	CName                              = "sync/peerPool"
	maxSimultaneousOperationsPerStream = 10
)

var log = logger.NewNamed("peerPool")

var (
	ErrPoolClosed   = errors.New("peer pool is closed")
	ErrPeerNotFound = errors.New("peer not found")
)

func NewPool() Pool {
	return &pool{closed: true}
}

type Handler func(ctx context.Context, msg *Message) (err error)

type Pool interface {
	DialAndAddPeer(ctx context.Context, id string) (err error)
	AddAndReadPeer(peer peer.Peer) (err error)
	AddHandler(msgType syncproto.MessageType, h Handler)
	AddPeerIdToGroup(peerId, groupId string) (err error)
	RemovePeerIdFromGroup(peerId, groupId string) (err error)

	SendAndWait(ctx context.Context, peerId string, msg *syncproto.Message) (err error)
	SendAndWaitResponse(ctx context.Context, id string, s *syncproto.Message) (resp *Message, err error)
	Broadcast(ctx context.Context, groupId string, msg *syncproto.Message) (err error)

	app.ComponentRunnable
}

type pool struct {
	peersById       map[string]*peerEntry
	waiters         *waiters
	handlers        map[syncproto.MessageType][]Handler
	peersIdsByGroup map[string][]string

	dialer dialer.Dialer

	closed bool
	mu     sync.RWMutex
	wg     *sync.WaitGroup
}

func (p *pool) Init(ctx context.Context, a *app.App) (err error) {
	p.peersById = map[string]*peerEntry{}
	p.handlers = map[syncproto.MessageType][]Handler{}
	p.peersIdsByGroup = map[string][]string{}
	p.waiters = &waiters{waiters: map[uint64]*waiter{}}
	p.dialer = a.MustComponent(dialer.CName).(dialer.Dialer)
	p.wg = &sync.WaitGroup{}
	return nil
}

func (p *pool) Name() (name string) {
	return CName
}

func (p *pool) Run(ctx context.Context) (err error) {
	p.closed = false
	return nil
}

func (p *pool) AddHandler(msgType syncproto.MessageType, h Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		// unable to add handler after Run
		return
	}
	p.handlers[msgType] = append(p.handlers[msgType], h)
}

func (p *pool) DialAndAddPeer(ctx context.Context, peerId string) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrPoolClosed
	}
	if _, ok := p.peersById[peerId]; ok {
		return nil
	}
	peer, err := p.dialer.Dial(ctx, peerId)
	if err != nil {
		return
	}
	p.peersById[peer.Id()] = &peerEntry{
		peer: peer,
	}
	p.wg.Add(1)
	go p.readPeerLoop(peer)
	return nil
}

func (p *pool) AddAndReadPeer(peer peer.Peer) (err error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrPoolClosed
	}
	p.peersById[peer.Id()] = &peerEntry{
		peer: peer,
	}
	p.wg.Add(1)
	p.mu.Unlock()
	return p.readPeerLoop(peer)
}

func (p *pool) AddPeerIdToGroup(peerId, groupId string) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	peer, ok := p.peersById[peerId]
	if !ok {
		return ErrPeerNotFound
	}
	if slice.FindPos(peer.groupIds, groupId) != -1 {
		return nil
	}
	peer.addGroup(groupId)
	p.peersIdsByGroup[groupId] = append(p.peersIdsByGroup[groupId], peerId)
	return
}

func (p *pool) RemovePeerIdFromGroup(peerId, groupId string) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	peer, ok := p.peersById[peerId]
	if !ok {
		return ErrPeerNotFound
	}
	if slice.FindPos(peer.groupIds, groupId) == -1 {
		return nil
	}
	peer.removeGroup(groupId)
	p.peersIdsByGroup[groupId] = slice.Remove(p.peersIdsByGroup[groupId], peerId)
	return
}

func (p *pool) SendAndWait(ctx context.Context, peerId string, msg *syncproto.Message) (err error) {
	resp, err := p.SendAndWaitResponse(ctx, peerId, msg)
	if err != nil {
		return
	}
	return resp.IsAck()
}

func (p *pool) SendAndWaitResponse(ctx context.Context, peerId string, msg *syncproto.Message) (resp *Message, err error) {
	defer func() {
		if err != nil {
			log.With(
				zap.String("peerId", peerId),
				zap.String("header", msg.GetHeader().String())).
				Error("failed sending message to peer", zap.Error(err))
		} else {
			log.With(
				zap.String("peerId", peerId),
				zap.String("header", msg.GetHeader().String())).
				Debug("sent message to peer")
		}
	}()

	p.mu.RLock()
	peer := p.peersById[peerId]
	p.mu.RUnlock()
	if peer == nil {
		err = ErrPeerNotFound
		return
	}

	repId := p.waiters.NewReplyId()
	msg.GetHeader().RequestId = repId
	ch := make(chan Reply, 1)

	log.With(zap.Uint64("reply id", repId)).Debug("adding waiter for reply id")
	p.waiters.Add(repId, &waiter{ch: ch})
	defer p.waiters.Remove(repId)

	if err = peer.peer.Send(msg); err != nil {
		return
	}
	select {
	case rep := <-ch:
		if rep.Error != nil {
			err = rep.Error
			return
		}
		resp = rep.Message
		return
	case <-ctx.Done():
		log.Debug("context done in SendAndWait")
		err = ctx.Err()
	}
	return
}

func (p *pool) Broadcast(ctx context.Context, groupId string, msg *syncproto.Message) (err error) {
	//TODO implement me
	panic("implement me")
}

func (p *pool) readPeerLoop(peer peer.Peer) (err error) {
	defer p.wg.Done()

	limiter := make(chan struct{}, maxSimultaneousOperationsPerStream)
	for i := 0; i < maxSimultaneousOperationsPerStream; i++ {
		limiter <- struct{}{}
	}
Loop:
	for {
		msg, err := peer.Recv()
		if err != nil {
			log.Debug("peer receive error", zap.Error(err), zap.String("peerId", peer.Id()))
			break
		}
		select {
		case <-limiter:
		case <-peer.Context().Done():
			break Loop
		}
		go func() {
			defer func() {
				limiter <- struct{}{}
			}()
			p.handleMessage(peer, msg)
		}()
	}
	if err = p.removePeer(peer.Id()); err != nil {
		log.Error("remove peer error", zap.String("peerId", peer.Id()), zap.Error(err))
	}
	return
}

func (p *pool) removePeer(peerId string) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.peersById[peerId]
	if !ok {
		return ErrPeerNotFound
	}
	delete(p.peersById, peerId)
	return
}

func (p *pool) handleMessage(peer peer.Peer, msg *syncproto.Message) {
	log.With(zap.String("peerId", peer.Id()), zap.String("header", msg.GetHeader().String())).
		Debug("received message from peer")
	replyId := msg.GetHeader().GetReplyId()
	if replyId != 0 {
		if !p.waiters.Send(replyId, Reply{
			PeerInfo: peer.Info(),
			Message: &Message{
				Message: msg,
				peer:    peer,
			},
		}) {
			log.Debug("received reply with unknown (or expired) replyId", zap.Uint64("replyId", replyId), zap.String("header", msg.GetHeader().String()))
		}
		return
	}
	handlers := p.handlers[msg.GetHeader().GetType()]
	if len(handlers) == 0 {
		log.With(zap.String("peerId", peer.Id())).Debug("no handlers for such message")
		return
	}

	message := &Message{Message: msg, peer: peer}

	for _, h := range handlers {
		if err := h(peer.Context(), message); err != nil {
			log.Error("handle message error", zap.Error(err))
		}
	}
}

func (p *pool) Close(ctx context.Context) (err error) {
	p.mu.Lock()
	for _, peer := range p.peersById {
		peer.peer.Close()
	}
	wg := p.wg
	p.mu.Unlock()
	if wg != nil {
		wg.Wait()
	}
	return nil
}

type waiter struct {
	sent int
	ch   chan<- Reply
}

type waiters struct {
	waiters  map[uint64]*waiter
	replySeq uint64
	mu       sync.Mutex
}

func (w *waiters) Send(replyId uint64, r Reply) (ok bool) {
	w.mu.Lock()
	wait := w.waiters[replyId]
	if wait == nil {
		w.mu.Unlock()
		return false
	}
	wait.sent++
	var lastMessage = wait.sent == cap(wait.ch)
	if lastMessage {
		delete(w.waiters, replyId)
	}
	w.mu.Unlock()
	wait.ch <- r
	if lastMessage {
		close(wait.ch)
	}
	return true
}

func (w *waiters) Add(replyId uint64, wait *waiter) {
	w.mu.Lock()
	w.waiters[replyId] = wait
	w.mu.Unlock()
}

func (w *waiters) Remove(id uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.waiters[id]; ok {
		delete(w.waiters, id)
		return nil
	}
	return fmt.Errorf("waiter not found")
}

func (w *waiters) NewReplyId() uint64 {
	res := atomic.AddUint64(&w.replySeq, 1)
	if res == 0 {
		return w.NewReplyId()
	}
	return res
}
