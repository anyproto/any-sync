package space

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ldiff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/configuration"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/space/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/space/spacesync"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

type Space interface {
	Id() string

	HeadSync(ctx context.Context, req *spacesync.HeadSyncRequest) (*spacesync.HeadSyncResponse, error)

	Close() error
}

//

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

func (s *space) HeadSync(ctx context.Context, req *spacesync.HeadSyncRequest) (*spacesync.HeadSyncResponse, error) {
	return remotediff.HandlerRangeRequest(ctx, s.diff, req)
}

func (s *space) testFill() {
	var n = 1000
	var els = make([]ldiff.Element, 0, n)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < n; i++ {
		if rand.Intn(n) > 2 {
			id := fmt.Sprintf("%s.%d", s.id, i)
			head := "head." + id
			if rand.Intn(n) > n-100 {
				head += ".modified"
			}
			el := ldiff.Element{
				Id:   id,
				Head: head,
			}
			els = append(els, el)
		}
	}
	s.diff.Set(els...)
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
				return
			case <-ticker.C:
				doSync()
			}
		}
	}
}

func (s *space) sync(ctx context.Context) error {
	peers, err := s.getPeers(ctx)
	if err != nil {
		return err
	}
	for _, p := range peers {
		if err := s.syncWithPeer(ctx, p); err != nil {
			log.Error("can't sync with peer", zap.String("peer", p.Id()), zap.Error(err))
		}
	}
	return nil
}

func (s *space) syncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	cl := spacesync.NewDRPCSpaceClient(p)
	rdiff := remotediff.NewRemoteDiff(s.id, cl)
	newIds, changedIds, removedIds, err := s.diff.Diff(ctx, rdiff)
	if err != nil {
		return nil
	}
	log.Info("sync done:", zap.Int("newIds", len(newIds)), zap.Int("changedIds", len(changedIds)), zap.Int("removedIds", len(removedIds)))
	return
}

func (s *space) getPeers(ctx context.Context) (peers []peer.Peer, err error) {
	if s.conf.IsResponsible(s.id) {
		return s.conf.AllPeers(ctx, s.id)
	} else {
		var p peer.Peer
		p, err = s.conf.OnePeer(ctx, s.id)
		if err != nil {
			return nil, err
		}
		return []peer.Peer{p}, nil
	}
}

func (s *space) Close() error {
	s.syncCancel()
	<-s.syncLoopDone
	return nil
}
