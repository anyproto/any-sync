package filestorage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/filestorage/badgerfilestore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	blocks "github.com/ipfs/go-block-format"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"
)

const syncerOpBatch = 10

type syncer struct {
	ps   *proxyStore
	done chan struct{}
}

func (s *syncer) run(ctx context.Context) {
	defer close(s.done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		case <-s.ps.index.HasWorkCh():
		}
		for s.sync(ctx) > 0 {
		}
	}
}

func (s *syncer) sync(ctx context.Context) (doneCount int32) {
	cids, err := s.ps.index.List(syncerOpBatch)
	if err != nil {
		log.Error("index list error", zap.Error(err))
		return
	}
	defer cids.Release()
	l := cids.Len()
	log.Debug("remote file sync, got tasks to sync", zap.Int("count", l))
	if l == 0 {
		return
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var wg sync.WaitGroup
	var doneAtomic atomic.Int32
	for _, sOps := range cids.SpaceOps {
		if len(sOps.Load) > 0 {
			wg.Add(1)
			go func(opt badgerfilestore.SpaceCidOps) {
				defer wg.Done()
				doneAtomic.Add(s.load(ctx, opt))
			}(sOps)
		}
		if len(sOps.Delete) > 0 {
			wg.Add(1)
			go func(opt badgerfilestore.SpaceCidOps) {
				defer wg.Done()
				doneAtomic.Add(s.delete(ctx, opt))
			}(sOps)
		}
		if len(sOps.Add) > 0 {
			wg.Add(1)
			go func(opt badgerfilestore.SpaceCidOps) {
				defer wg.Done()
				doneAtomic.Add(s.add(ctx, opt))
			}(sOps)
		}
	}
	wg.Wait()
	return doneAtomic.Load()
}

func (s *syncer) load(ctx context.Context, spaceOps badgerfilestore.SpaceCidOps) (doneCount int32) {
	ctx = fileblockstore.CtxWithSpaceId(ctx, spaceOps.SpaceId)
	res := s.ps.origin.GetMany(ctx, spaceOps.Load)
	doneCids := badgerfilestore.NewCids()
	defer doneCids.Release()
	for b := range res {
		if err := s.ps.cache.Add(ctx, []blocks.Block{b}); err != nil {
			log.Error("syncer: can't add to local store", zap.Error(err))
			continue
		}
		doneCids.Add(spaceOps.SpaceId, badgerfilestore.OpLoad, b.Cid())
	}
	if err := s.ps.index.Done(doneCids); err != nil {
		log.Error("syncer: index.Done error", zap.Error(err))
		return
	}
	doneCount = int32(doneCids.Len())
	log.Info("successfully loaded cids", zap.Int32("count", doneCount))
	return
}

func (s *syncer) add(ctx context.Context, spaceOps badgerfilestore.SpaceCidOps) (doneCount int32) {
	doneCids := badgerfilestore.NewCids()
	defer doneCids.Release()
	res := s.ps.cache.GetMany(ctx, spaceOps.Add)
	var bs []blocks.Block
	for b := range res {
		bs = append(bs, b)
		doneCids.Add(spaceOps.SpaceId, badgerfilestore.OpAdd, b.Cid())
	}
	ctx = fileblockstore.CtxWithSpaceId(ctx, spaceOps.SpaceId)
	if err := s.ps.origin.Add(ctx, bs); err != nil {
		log.Debug("syncer: can't add to remote store", zap.Error(err))
		return
	}

	if err := s.ps.index.Done(doneCids); err != nil {
		log.Error("syncer: index.Done error", zap.Error(err))
		return
	}
	doneCount = int32(doneCids.Len())
	log.Info("successfully added cids", zap.Int32("count", doneCount), zap.Stringers("cids", doneCids.SpaceOps[0].Add))
	return
}

func (s *syncer) delete(ctx context.Context, spaceOps badgerfilestore.SpaceCidOps) (doneCount int32) {
	doneCids := badgerfilestore.NewCids()
	defer doneCids.Release()
	ctx = fileblockstore.CtxWithSpaceId(ctx, spaceOps.SpaceId)
	for _, c := range spaceOps.Delete {
		if err := s.ps.origin.Delete(ctx, c); err != nil {
			log.Debug("syncer: can't remove from remote", zap.Error(err))
			continue
		}
		doneCids.Add(spaceOps.SpaceId, badgerfilestore.OpDelete, c)
	}
	if err := s.ps.index.Done(doneCids); err != nil {
		log.Error("syncer: index.Done error", zap.Error(err))
	}
	doneCount = int32(doneCids.Len())
	log.Info("successfully removed cids", zap.Int32("count", doneCount))
	return
}
