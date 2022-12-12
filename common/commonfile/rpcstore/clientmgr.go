package rpcstore

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

const (
	maxConnections = 10
	maxTasks       = 1000
)

var (
	clientCreateTimeout = time.Second * 10
)

func newClientManager(s *service) *clientManager {
	cm := &clientManager{
		mb: mb.New[*task](maxTasks),
		ocache: ocache.New(
			func(ctx context.Context, id string) (value ocache.Object, err error) {
				return nil, fmt.Errorf("load func shouldn't be used")
			},
			ocache.WithTTL(time.Minute*5),
			ocache.WithRefCounter(false),
			ocache.WithLogger(log.Sugar()),
			ocache.WithGCPeriod(0),
		),
		s: s,
	}
	cm.ctx, cm.ctxCancel = context.WithCancel(context.Background())
	go cm.checkPeerLoop()
	return cm
}

// clientManager manages clients, removes unused ones, and adds new ones if necessary
type clientManager struct {
	mb        *mb.MB[*task]
	ctx       context.Context
	ctxCancel context.CancelFunc
	ocache    ocache.OCache

	s  *service
	mu sync.RWMutex
}

func (m *clientManager) Add(ctx context.Context, ts ...*task) (err error) {
	return m.mb.Add(ctx, ts...)
}

func (m *clientManager) checkPeerLoop() {
	m.checkPeers()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkPeers()
		}
	}
}

func (m *clientManager) checkPeers() {
	// start GC to remove unused clients
	m.ocache.GC()
	if m.ocache.Len() >= maxConnections {
		// reached connection limit, can't add new peers
		return
	}
	if m.ocache.Len() != 0 && m.mb.Len() == 0 {
		// has empty queue, no need new peers
		return
	}

	// try to add new peers
	peerIds := m.s.filePeers()
	rand.Shuffle(len(peerIds), func(i, j int) {
		peerIds[i], peerIds[j] = peerIds[j], peerIds[i]
	})
	for _, peerId := range peerIds {
		if _, cerr := m.ocache.Pick(m.ctx, peerId); cerr == ocache.ErrNotExists {
			ctx, cancel := context.WithTimeout(m.ctx, clientCreateTimeout)
			cl, err := newClient(ctx, m.s, peerId, m.mb)
			if err != nil {
				log.Info("can't create client", zap.Error(err))
				cancel()
				continue
			}
			_ = m.ocache.Add(peerId, cl)
			cancel()
			return
		}
	}
}

func (m *clientManager) Close() (err error) {
	m.ctxCancel()
	if err = m.mb.Close(); err != nil {
		log.Error("mb close error", zap.Error(err))
	}
	return m.ocache.Close()
}
