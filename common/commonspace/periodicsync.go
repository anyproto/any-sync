package commonspace

import (
	"context"
	"go.uber.org/zap"
	"time"
)

func newPeriodicSync(periodSeconds int, sync func(ctx context.Context) error, l *zap.Logger) *periodicSync {
	ctx, cancel := context.WithCancel(context.Background())
	ps := &periodicSync{
		log:          l,
		sync:         sync,
		syncCtx:      ctx,
		syncCancel:   cancel,
		syncLoopDone: make(chan struct{}),
	}
	go ps.syncLoop(periodSeconds)
	return ps
}

type periodicSync struct {
	log          *zap.Logger
	sync         func(ctx context.Context) error
	syncCtx      context.Context
	syncCancel   context.CancelFunc
	syncLoopDone chan struct{}
}

func (p *periodicSync) syncLoop(periodSeconds int) {
	period := time.Duration(periodSeconds) * time.Second
	defer close(p.syncLoopDone)
	doSync := func() {
		ctx, cancel := context.WithTimeout(p.syncCtx, time.Minute)
		defer cancel()
		if err := p.sync(ctx); err != nil {
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
