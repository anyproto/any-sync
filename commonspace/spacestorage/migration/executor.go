package migration

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

var log = logger.NewNamed("common.spacestorage.migration")

func newMigratePool(ctx context.Context, workers, maxSize int) *migratePool {
	ss := &migratePool{
		workers: workers,
		ctx:     ctx,
		wg:      newContextWaitGroup(ctx),
		batch:   mb.New[func()](maxSize),
	}
	return ss
}

type migratePool struct {
	workers int
	batch   *mb.MB[func()]
	close   chan struct{}
	ctx     context.Context
	wg      *contextWaitGroup
}

func (mp *migratePool) Add(ctx context.Context, f ...func()) (err error) {
	err = mp.batch.Add(ctx, f...)
	if err == nil {
		mp.wg.Add(1)
	}
	return err
}

func (mp *migratePool) TryAdd(f ...func()) (err error) {
	err = mp.batch.TryAdd(f...)
	if err == nil {
		mp.wg.Add(1)
	}
	return err
}

func (mp *migratePool) Run() {
	for i := 0; i < mp.workers; i++ {
		go mp.sendLoop()
	}
}

func (mp *migratePool) sendLoop() {
	for {
		f, err := mp.batch.WaitOne(mp.ctx)
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		f()
		mp.wg.Done()
	}
}

func (mp *migratePool) Wait() (err error) {
	err = mp.wg.Wait()
	mp.batch.Close()
	return err
}
