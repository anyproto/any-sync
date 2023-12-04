package requestmanager

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestPool_Add(t *testing.T) {
	t.Run("add many same key", func(t *testing.T) {
		firstVal := atomic.Int32{}
		secondVal := atomic.Int32{}
		ch := make(chan struct{})
		setBlock := func() {
			<-ch
		}
		rp := NewRequestPool(2, 2)
		err := rp.TryAdd("1", setBlock)
		require.NoError(t, err)
		err = rp.TryAdd("2", setBlock)
		require.NoError(t, err)
		rp.Run()
		// wait until setBlock is executed
		time.Sleep(time.Millisecond * 100)
		for i := 0; i <= 100; i++ {
			tmp := i
			err := rp.TryAdd("1", func() {
				firstVal.Store(int32(tmp))
			})
			require.NoError(t, err)
			err = rp.TryAdd("2", func() {
				secondVal.Store(int32(tmp))
			})
			require.NoError(t, err)
		}
		close(ch)
		// wait until rp runs all tasks
		time.Sleep(time.Millisecond * 100)
		rp.Close()
		require.Equal(t, int32(100), firstVal.Load())
		require.Equal(t, int32(100), secondVal.Load())
	})
	t.Run("error does not save func", func(t *testing.T) {
		firstVal := atomic.Int32{}
		secondVal := atomic.Int32{}
		thirdVal := atomic.Int32{}
		rp := NewRequestPool(2, 2)
		err := rp.TryAdd("1", func() {
			firstVal.Store(1)
		})
		require.NoError(t, err)
		err = rp.TryAdd("2", func() {
			secondVal.Store(2)
		})
		require.NoError(t, err)
		err = rp.TryAdd("3", func() {
			thirdVal.Store(3)
		})
		require.Error(t, err)
		require.Empty(t, rp.entries["3"])
	})
}
