//go:generate mockgen -destination mock_periodicsync/mock_periodicsync.go github.com/anyproto/any-sync/util/periodicsync PeriodicSync
package periodicsync

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

type PeriodicSync interface {
	Run()
	Close()
}

type SyncerFunc func(ctx context.Context) error

func NewPeriodicSync(periodSeconds int, timeout time.Duration, caller SyncerFunc, l logger.CtxLogger) PeriodicSync {
	return NewPeriodicSyncDuration(time.Duration(periodSeconds)*time.Second, timeout, caller, l)
}

func NewPeriodicSyncDuration(periodicLoopInterval, timeout time.Duration, caller SyncerFunc, l logger.CtxLogger) PeriodicSync {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.CtxWithFields(ctx, zap.String("rootOp", "periodicCall"))
	return &periodicCall{
		caller:     caller,
		log:        l,
		loopCtx:    ctx,
		loopCancel: cancel,
		loopDone:   make(chan struct{}),
		period:     periodicLoopInterval,
		timeout:    timeout,
	}
}

type periodicCall struct {
	log        logger.CtxLogger
	caller     SyncerFunc
	loopCtx    context.Context
	loopCancel context.CancelFunc
	loopDone   chan struct{}
	period     time.Duration
	timeout    time.Duration
	isRunning  atomic.Bool
}

func (p *periodicCall) Run() {
	p.isRunning.Store(true)
	go p.loop(p.period)
}

func (p *periodicCall) loop(period time.Duration) {
	defer close(p.loopDone)
	doCall := func() {
		ctx := p.loopCtx
		if p.timeout != 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(p.loopCtx, p.timeout)
			defer cancel()
		}
		if err := p.caller(ctx); err != nil {
			p.log.Warn("periodic call error", zap.Error(err))
		}
	}
	doCall()
	if period > 0 {
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-p.loopCtx.Done():
				return
			case <-ticker.C:
				doCall()
			}
		}
	}
}

func (p *periodicCall) Close() {
	if !p.isRunning.Load() {
		return
	}
	p.loopCancel()
	<-p.loopDone
}
