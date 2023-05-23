package objectsync

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type LastUsage interface {
	LastUsage() time.Time
}

// MessagePool can be made generic to work with different streams
type MessagePool interface {
	LastUsage
	synchandler.SyncHandler
	peermanager.PeerManager
	SendSync(ctx context.Context, peerId string, message *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
}

type MessageHandler func(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error)

type responseWaiter struct {
	ch chan *spacesyncproto.ObjectSyncMessage
}

type messagePool struct {
	sync.Mutex
	peermanager.PeerManager
	messageHandler MessageHandler
	waiters        map[string]responseWaiter
	waitersMx      sync.Mutex
	counter        atomic.Uint64
	lastUsage      atomic.Int64
}

func newMessagePool(peerManager peermanager.PeerManager, messageHandler MessageHandler) MessagePool {
	s := &messagePool{
		PeerManager:    peerManager,
		messageHandler: messageHandler,
		waiters:        make(map[string]responseWaiter),
	}
	return s
}

func (s *messagePool) SendSync(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	s.updateLastUsage()
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()
	newCounter := s.counter.Add(1)
	msg.RequestId = genReplyKey(peerId, msg.ObjectId, newCounter)
	log.InfoCtx(ctx, "mpool sendSync", zap.String("requestId", msg.RequestId))
	s.waitersMx.Lock()
	waiter := responseWaiter{
		ch: make(chan *spacesyncproto.ObjectSyncMessage, 1),
	}
	s.waiters[msg.RequestId] = waiter
	s.waitersMx.Unlock()

	err = s.SendPeer(ctx, peerId, msg)
	if err != nil {
		return
	}
	select {
	case <-ctx.Done():
		s.waitersMx.Lock()
		delete(s.waiters, msg.RequestId)
		s.waitersMx.Unlock()

		log.With(zap.String("requestId", msg.RequestId)).DebugCtx(ctx, "time elapsed when waiting")
		err = fmt.Errorf("sendSync context error: %v", ctx.Err())
	case reply = <-waiter.ch:
		// success
	}
	return
}

func (s *messagePool) SendPeer(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	s.updateLastUsage()
	return s.PeerManager.SendPeer(ctx, peerId, msg)
}

func (s *messagePool) Broadcast(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	s.updateLastUsage()
	return s.PeerManager.Broadcast(ctx, msg)
}

func (s *messagePool) HandleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	s.updateLastUsage()
	if msg.ReplyId != "" {
		log.InfoCtx(ctx, "mpool receive reply", zap.String("replyId", msg.ReplyId))
		// we got reply, send it to waiter
		if s.stopWaiter(msg) {
			return
		}
		log.WarnCtx(ctx, "reply id does not exist", zap.String("replyId", msg.ReplyId))
		return
	}
	return s.messageHandler(ctx, senderId, msg)
}

func (s *messagePool) LastUsage() time.Time {
	return time.Unix(s.lastUsage.Load(), 0)
}

func (s *messagePool) updateLastUsage() {
	s.lastUsage.Store(time.Now().Unix())
}

func (s *messagePool) stopWaiter(msg *spacesyncproto.ObjectSyncMessage) bool {
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

func genReplyKey(peerId, treeId string, counter uint64) string {
	b := &strings.Builder{}
	b.WriteString(peerId)
	b.WriteString(".")
	b.WriteString(treeId)
	b.WriteString(".")
	b.WriteString(strconv.FormatUint(counter, 36))
	return b.String()
}
