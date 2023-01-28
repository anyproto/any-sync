//go:generate mockgen -destination mock_periodicsync/mock_periodicsync.go github.com/anytypeio/any-sync/util/periodicsync PeriodicSync
package periodicsync

import (
	"context"
	"github.com/anytypeio/any-sync/app/logger"
	"go.uber.org/zap"
	"time"
)

type PeriodicSync interface {
	Run()
	Close()
}

type SyncerFunc func(ctx context.Context) error

func NewPeriodicSync(periodSeconds int, timeout time.Duration, syncer SyncerFunc, l logger.CtxLogger) PeriodicSync {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.CtxWithFields(ctx, zap.String("rootOp", "periodicSync"))
	return &periodicSync{
		syncer:        syncer,
		log:           l,
		syncCtx:       ctx,
		syncCancel:    cancel,
		syncLoopDone:  make(chan struct{}),
		periodSeconds: periodSeconds,
		timeout:       timeout,
	}
}

type periodicSync struct {
	log           logger.CtxLogger
	syncer        SyncerFunc
	syncCtx       context.Context
	syncCancel    context.CancelFunc
	syncLoopDone  chan struct{}
	periodSeconds int
	timeout       time.Duration
}

func (p *periodicSync) Run() {
	go p.syncLoop(p.periodSeconds)
}

func (p *periodicSync) syncLoop(periodSeconds int) {
	period := time.Duration(periodSeconds) * time.Second
	defer close(p.syncLoopDone)
	doSync := func() {
		ctx := p.syncCtx
		if p.timeout != 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(p.syncCtx, p.timeout)
			defer cancel()
		}
		if err := p.syncer(ctx); err != nil {
			p.log.Warn("periodic sync error", zap.Error(err))
		}
	}
	doSync()
	if period > 0 {
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-p.syncCtx.Done():
				return
			case <-ticker.C:
				doSync()
			}
		}
	}
}

func (p *periodicSync) Close() {
	p.syncCancel()
	<-p.syncLoopDone
}
