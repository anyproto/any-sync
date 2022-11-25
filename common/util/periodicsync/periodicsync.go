//go:generate mockgen -destination mock_periodicsync/mock_periodicsync.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/periodicsync PeriodicSync
package periodicsync

import (
	"context"
	"go.uber.org/zap"
	"time"
)

type PeriodicSync interface {
	Run()
	Close()
}

type SyncerFunc func(ctx context.Context) error

func NewPeriodicSync(periodSeconds int, syncer SyncerFunc, l *zap.Logger) PeriodicSync {
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
	syncer        SyncerFunc
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
