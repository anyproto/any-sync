package requestmanager

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestPool_Add(t *testing.T) {
	t.Run("add many same key", func(t *testing.T) {
		removed := atomic.Int32{}
		firstVal := atomic.Int32{}
		secondVal := atomic.Int32{}
		ch := make(chan struct{})
		setBlock := func() {
			<-ch
		}
		onRemove := func() {
			removed.Add(1)
		}
		rp := newRequestPool(2, 2)
		err := rp.TryAdd("1", setBlock, onRemove)
		require.NoError(t, err)
		err = rp.TryAdd("2", setBlock, onRemove)
		require.NoError(t, err)
		rp.Run()
		// wait until setBlock is executed
		time.Sleep(time.Millisecond * 100)
		lenReq := 100
		for i := 0; i < lenReq; i++ {
			tmp := i
			err := rp.TryAdd("1", func() {
				firstVal.Store(int32(tmp))
			}, onRemove)
			require.NoError(t, err)
			err = rp.TryAdd("2", func() {
				secondVal.Store(int32(tmp))
			}, onRemove)
			require.NoError(t, err)
		}
		close(ch)
		// wait until rp runs all tasks
		time.Sleep(time.Millisecond * 100)
		rp.Close()
		require.Equal(t, int32(lenReq-1), firstVal.Load())
		require.Equal(t, int32(lenReq-1), secondVal.Load())
		require.Equal(t, int32(2*lenReq+2), removed.Load())
	})
	t.Run("error does not save func", func(t *testing.T) {
		removed := atomic.Int32{}
		firstVal := atomic.Int32{}
		secondVal := atomic.Int32{}
		thirdVal := atomic.Int32{}
		onRemove := func() {
			removed.Add(1)
		}
		rp := newRequestPool(2, 2)
		err := rp.TryAdd("1", func() {
			firstVal.Store(1)
		}, onRemove)
		require.NoError(t, err)
		err = rp.TryAdd("2", func() {
			secondVal.Store(2)
		}, onRemove)
		require.NoError(t, err)
		err = rp.TryAdd("3", func() {
			thirdVal.Store(3)
		}, onRemove)
		require.Error(t, err)
		require.Empty(t, rp.entries["3"])
		rp.Run()
		time.Sleep(100 * time.Millisecond)
		rp.Close()
		require.Equal(t, int32(3), removed.Load())
		require.Equal(t, int32(1), firstVal.Load())
		require.Equal(t, int32(2), secondVal.Load())
	})
}
