package diffservice

import (
	"context"
	"go.uber.org/zap"
	"time"
)

type PeriodicSync interface {
	Run()
	Close()
}

func newPeriodicSync(periodSeconds int, syncer DiffSyncer, l *zap.Logger) *periodicSync {
	ctx, cancel := context.WithCancel(context.Background())
	return &periodicSync{
		syncer:        syncer,
		log:           l,
		syncCtx:       ctx,
		syncCancel:    cancel,
		syncLoopDone:  make(chan struct{}),
		periodSeconds: periodSeconds,
	}
}

type periodicSync struct {
	log           *zap.Logger
	syncer        DiffSyncer
	syncCtx       context.Context
	syncCancel    context.CancelFunc
	syncLoopDone  chan struct{}
	periodSeconds int
}

func (p *periodicSync) Run() {
	go p.syncLoop(p.periodSeconds)
}

func (p *periodicSync) syncLoop(periodSeconds int) {
	period := time.Duration(periodSeconds) * time.Second
	defer close(p.syncLoopDone)
	doSync := func() {
		ctx, cancel := context.WithTimeout(p.syncCtx, time.Minute)
		defer cancel()
		if err := p.syncer.Sync(ctx); err != nil {
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
