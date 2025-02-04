package migration

import (
	"context"
	"sync"
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
	err := pool.TryAdd(func() {
		<-block
	})
	require.NoError(t, err)
	require.Error(t, pool.TryAdd(func() {}))
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
