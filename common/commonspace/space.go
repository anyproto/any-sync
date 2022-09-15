package commonspace

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ldiff"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

type Space interface {
	Id() string

	SpaceSyncRpc() RpcHandler
	SyncService() syncservice.SyncService

	Close() error
}

type space struct {
	id    string
	nconf nodeconf.Configuration
	conf  config.Space
	diff  ldiff.Diff
	mu    sync.RWMutex

	rpc          *rpcHandler
	periodicSync *periodicSync
	syncService  syncservice.SyncService
}

func (s *space) Id() string {
	return s.id
}

func (s *space) Init(ctx context.Context) error {
	s.diff = ldiff.New(16, 16)
	s.periodicSync = newPeriodicSync(s.conf.SyncPeriod, s.sync, log.With(zap.String("spaceId", s.id)))
	s.rpc = &rpcHandler{s: s}
	s.testFill()
	return nil
}

func (s *space) SpaceSyncRpc() RpcHandler {
	return s.rpc
}

func (s *space) SyncService() syncservice.SyncService {
	return s.syncService
}

func (s *space) testFill() {
	var n = 1000
	var els = make([]ldiff.Element, 0, n)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < n; i++ {
		if rand.Intn(n) > 2 {
			id := fmt.Sprintf("%s.%d", s.id, i)
			head := "head." + id
			if rand.Intn(n) > n-10 {
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

func (s *space) sync(ctx context.Context) error {
	st := time.Now()
	peers, err := s.getPeers(ctx)
	if err != nil {
		return err
	}
	for _, p := range peers {
		if err := s.syncWithPeer(ctx, p); err != nil {
			log.Error("can't sync with peer", zap.String("peer", p.Id()), zap.Error(err))
		}
	}
	log.Info("synced", zap.String("spaceId", s.id), zap.Duration("dur", time.Since(st)))
	return nil
}

func (s *space) syncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	cl := spacesyncproto.NewDRPCSpaceClient(p)
	rdiff := remotediff.NewRemoteDiff(s.id, cl)
	newIds, changedIds, removedIds, err := s.diff.Diff(ctx, rdiff)
	if err != nil {
		return nil
	}
	log.Info("sync done:", zap.Int("newIds", len(newIds)), zap.Int("changedIds", len(changedIds)), zap.Int("removedIds", len(removedIds)))
	return
}

func (s *space) getPeers(ctx context.Context) (peers []peer.Peer, err error) {
	if s.nconf.IsResponsible(s.id) {
		return s.nconf.AllPeers(ctx, s.id)
	} else {
		var p peer.Peer
		p, err = s.nconf.OnePeer(ctx, s.id)
		if err != nil {
			return nil, err
		}
		return []peer.Peer{p}, nil
	}
}

func (s *space) Close() error {
	s.periodicSync.Close()
	return nil
}
