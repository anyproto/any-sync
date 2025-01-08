package migration

import (
	"context"
	"sync"

	"github.com/cheggaaa/mb/v3"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
)

var log = logger.NewNamed("common.spacestorage.migration")

func newMigratePool(workers, maxSize int) *migratePool {
	ss := &migratePool{
		workers: workers,
		batch:   mb.New[func()](maxSize),
	}
	return ss
}

type migratePool struct {
	workers int
	batch   *mb.MB[func()]
	close   chan struct{}
	wg      sync.WaitGroup
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
		f, err := mp.batch.WaitOne(context.Background())
		mp.wg.Done()
		if err != nil {
			log.Debug("close send loop", zap.Error(err))
			return
		}
		f()
	}
}

func (mp *migratePool) Close() (err error) {
	mp.wg.Wait()
	return mp.batch.Close()
}
