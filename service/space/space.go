package space

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ldiff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/configuration"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/space/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/space/spacesync"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

type Space interface {
	Id() string
	Handle(ctx context.Context, msg *spacesync.Space) (repl *spacesync.Space, err error)
	Close() error
}

type space struct {
	id           string
	conf         configuration.Configuration
	diff         ldiff.Diff
	diffHandler  func()
	syncCtx      context.Context
	syncCancel   func()
	syncLoopDone chan struct{}
	s            *service
	mu           sync.RWMutex
}

func (s *space) Id() string {
	return s.id
}

func (s *space) Run(ctx context.Context) error {
	s.diff = ldiff.New(16, 16)
	s.syncCtx, s.syncCancel = context.WithCancel(context.Background())
	s.syncLoopDone = make(chan struct{})
	s.testFill()
	go s.syncLoop()
	return nil
}

func (s *space) testFill() {
	var n = 1000
	var els = make([]ldiff.Element, 0, n)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < n; i++ {
		if rand.Intn(n) > 2 {
			id := fmt.Sprintf("%s.%d", s.id, n)
			head := "head." + id
			if rand.Intn(n) > 2 {
				head += ".modified"
			}
			els = append(els, ldiff.Element{
				Id:   id,
				Head: head,
			})
		}
	}
	s.diff.Set(els...)
}

func (s *space) Handle(ctx context.Context, msg *spacesync.Space) (repl *spacesync.Space, err error) {
	if diffRange := msg.GetMessage().GetDiffRange(); diffRange != nil {
		resp, er := remotediff.HandlerRangeRequest(ctx, s.diff, diffRange)
		if er != nil {
			return nil, er
		}
		return &spacesync.Space{SpaceId: s.id, Message: &spacesync.Space_Content{
			Value: &spacesync.Space_Content_DiffRange{
				DiffRange: resp,
			},
		}}, nil
	}

	///

	///

	///

	return nil, fmt.Errorf("unexpected request")
}

func (s *space) syncLoop() {
	defer close(s.syncLoopDone)
	doSync := func() {
		ctx, cancel := context.WithTimeout(s.syncCtx, time.Minute)
		defer cancel()
		if err := s.sync(ctx); err != nil {
			log.Error("periodic sync error", zap.Error(err), zap.String("spaceId", s.id))
		}
	}
	doSync()
	if s.s.conf.SyncPeriod > 0 {
		ticker := time.NewTicker(time.Second * time.Duration(s.s.conf.SyncPeriod))
		defer ticker.Stop()
		for {
			select {
			case <-s.syncCtx.Done():
			case <-ticker.C:
				doSync()
			}
		}
	}
}

func (s *space) sync(ctx context.Context) error {
	peerIds, err := s.peerIds(ctx)
	if err != nil {
		return err
	}
	for _, peerId := range peerIds {
		if err := s.syncWithPeer(ctx, peerId); err != nil {
			log.Error("can't sync with peer", zap.String("peer", peerId), zap.Error(err))
		}
	}
	return nil
}

func (s *space) syncWithPeer(ctx context.Context, peerId string) (err error) {
	rdiff := remotediff.NewRemoteDiff(s.s.pool, peerId, s.id)
	newIds, changedIds, removedIds, err := s.diff.Diff(ctx, rdiff)
	if err != nil {
		return nil
	}
	log.Info("sync done:", zap.Strings("newIds", newIds), zap.Strings("changedIds", changedIds), zap.Strings("removedIds", removedIds))
	return
}

func (s *space) peerIds(ctx context.Context) (peerIds []string, err error) {
	if s.conf.IsResponsible(s.id) {
		peers, err := s.conf.AllPeers(ctx, s.id)
		if err != nil {
			return nil, err
		}
		for _, p := range peers {
			peerIds = append(peerIds, p.Id())
		}
	} else {
		peer, err := s.conf.OnePeer(ctx, s.id)
		if err != nil {
			return nil, err
		}
		peerIds = append(peerIds, peer.Id())
	}
	return
}

func (s *space) Close() error {
	s.syncCancel()
	<-s.syncLoopDone
	return nil
}
