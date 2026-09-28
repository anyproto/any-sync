package migration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigratePoolBasicFunctionality(t *testing.T) {
	ctx := context.Background()
	pool := newMigratePool(ctx, 2, 10)
	pool.Run()
	var (
		mu     sync.Mutex
		count  int
		wg     sync.WaitGroup
		nTasks = 5
	)
	wg.Add(nTasks)
	for i := 0; i < nTasks; i++ {
		err := pool.Add(ctx, func() {
			mu.Lock()
			count++
			mu.Unlock()
			wg.Done()
		})
		require.NoError(t, err)
	}
	wg.Wait()
	require.NoError(t, pool.Wait())
	require.Equal(t, nTasks, count)
}

func TestMigratePoolContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := newMigratePool(ctx, 1, 1)
	pool.Run()
	taskStarted := make(chan struct{})
	testProceed := make(chan struct{})
	err := pool.Add(ctx, func() {
		close(taskStarted)
		<-testProceed
	})
	require.NoError(t, err)
	<-taskStarted
	cancel()
	require.Error(t, pool.Wait())
	close(testProceed)
}

func TestMigratePoolTryAddWhenFull(t *testing.T) {
	ctx := context.Background()
	pool := newMigratePool(ctx, 1, 1)
	pool.Run()
	block := make(chan struct{})
	defer close(block)
	started := make(chan struct{})
	err := pool.TryAdd(func() {
		close(started)
		<-block
	})
	require.NoError(t, err)
	// the worker holds the first task, the second one fills the queue
	<-started
	require.NoError(t, pool.TryAdd(func() {}))
	require.Error(t, pool.TryAdd(func() {}))
}

func TestMigratePoolTaskDoneBeforeAddReturns(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < 2000; i++ {
		pool := newMigratePool(ctx, 4, 10)
		pool.Run()
		for j := 0; j < 10; j++ {
			require.NoError(t, pool.Add(ctx, func() {}))
		}
		require.NoError(t, pool.Wait())
	}
}

func TestMigratePoolAddMany(t *testing.T) {
	ctx := context.Background()
	pool := newMigratePool(ctx, 2, 10)
	pool.Run()
	var count atomic.Int32
	task := func() { count.Add(1) }
	require.NoError(t, pool.Add(ctx, task, task, task))
	require.NoError(t, pool.TryAdd(task, task))
	require.NoError(t, pool.Wait())
	require.Equal(t, int32(5), count.Load())
}

func TestContextWaitGroupNormalWait(t *testing.T) {
	ctx := context.Background()
	cwg := newContextWaitGroup(ctx)
	var wg sync.WaitGroup
	wg.Add(2)
	cwg.Add(2)
	go func() {
		defer wg.Done()
		cwg.Done()
	}()
	go func() {
		defer wg.Done()
		cwg.Done()
	}()

	wg.Wait()
	require.NoError(t, cwg.Wait())
}

func TestContextWaitGroupContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cwg := newContextWaitGroup(ctx)
	cwg.Add(1)
	cancel()
	require.Error(t, cwg.Wait())
}
