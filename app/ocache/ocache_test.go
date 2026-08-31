package ocache

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

type testObject struct {
	name           string
	closeErr       error
	closeCh        chan struct{}
	tryReturn      bool
	closeCalled    bool
	tryCloseCalled bool
}

func NewTestObject(name string, tryReturn bool, closeCh chan struct{}) *testObject {
	return &testObject{
		name:      name,
		closeCh:   closeCh,
		tryReturn: tryReturn,
	}
}

func (t *testObject) Close() (err error) {
	if t.closeCalled || (t.tryCloseCalled && t.tryReturn) {
		panic("close called twice")
	}
	t.closeCalled = true
	if t.closeCh != nil {
		<-t.closeCh
	}
	return t.closeErr
}

func (t *testObject) TryClose(objectTTL time.Duration) (res bool, err error) {
	if t.closeCalled || (t.tryCloseCalled && t.tryReturn) {
		panic("close called twice")
	}
	t.tryCloseCalled = true
	if t.closeCh != nil {
		<-t.closeCh
		return t.tryReturn, t.closeErr
	}
	return t.tryReturn, nil
}

func TestOCache_Get(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return &testObject{name: "test"}, nil
		})
		val, err := c.Get(context.TODO(), "test")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, "test", val.(*testObject).name)
		assert.Equal(t, 1, c.Len())
		assert.NoError(t, c.Close())
	})
	t.Run("error", func(t *testing.T) {
		tErr := errors.New("err")
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return nil, tErr
		})
		val, err := c.Get(context.TODO(), "test")
		require.Equal(t, tErr, err)
		require.Nil(t, val)
		assert.Equal(t, 0, c.Len())
		assert.NoError(t, c.Close())
	})
	t.Run("parallel load", func(t *testing.T) {
		var waitCh = make(chan struct{})
		var obj = &testObject{
			name: "test",
		}
		var calls uint32
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			atomic.AddUint32(&calls, 1)
			<-waitCh
			return obj, nil
		})

		var l = 10
		var res = make(chan struct{}, l)

		for i := 0; i < l; i++ {
			go func() {
				val, err := c.Get(context.TODO(), "id")
				require.NoError(t, err)
				assert.Equal(t, obj, val)
				res <- struct{}{}
			}()
		}
		time.Sleep(time.Millisecond * 10)
		close(waitCh)
		var timeout = time.After(time.Second)
		for i := 0; i < l; i++ {
			select {
			case <-res:
			case <-timeout:
				require.True(t, false, "timeout")
			}
		}
		assert.Equal(t, 1, c.Len())
		assert.Equal(t, uint32(1), calls)
		assert.NoError(t, c.Close())
	})
	t.Run("errClosed", func(t *testing.T) {
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return nil, errors.New("test")
		})
		require.NoError(t, c.Close())
		_, err := c.Get(context.TODO(), "id")
		assert.Equal(t, ErrClosed, err)
	})
	t.Run("context cancel", func(t *testing.T) {
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			time.Sleep(time.Second / 3)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return &testObject{
				name: "id",
			}, nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.Get(ctx, "id")
		assert.Equal(t, context.Canceled, err)
		assert.NoError(t, c.Close())
	})
	t.Run("value is nil", func(t *testing.T) {
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return nil, nil
		})

		value, err := c.Get(ctx, "id")
		assert.NotNil(t, err)
		assert.Nil(t, value)
		assert.NoError(t, c.Close())
	})
}

func TestOCache_GetForeignCtxRetry(t *testing.T) {
	t.Run("waiter survives first caller's cancellation", func(t *testing.T) {
		// The first caller owns the load's context; every concurrent Get
		// shares the load's result. Pre-fix, cancelling the first caller
		// failed the waiter with a context error it did not cause.
		var (
			started = make(chan struct{}, 2)
			release = make(chan struct{})
			calls   atomic.Int32
		)
		c := New(func(ctx context.Context, id string) (Object, error) {
			calls.Add(1)
			started <- struct{}{}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-release:
				return &testObject{name: id}, nil
			}
		})
		ownerCtx, cancelOwner := context.WithCancel(context.Background())
		ownerErr := make(chan error, 1)
		go func() {
			_, err := c.Get(ownerCtx, "id")
			ownerErr <- err
		}()
		<-started // the owner's load is in flight
		waiterErr := make(chan error, 1)
		go func() {
			val, err := c.Get(context.Background(), "id")
			if err == nil && val == nil {
				err = errors.New("nil value")
			}
			waiterErr <- err
		}()
		time.Sleep(time.Millisecond * 10) // let the waiter join the in-flight load
		cancelOwner()
		require.Equal(t, context.Canceled, <-ownerErr, "the owner's own cancellation is its error")
		<-started // the waiter retried: a fresh load, owned by its live ctx
		close(release)
		require.NoError(t, <-waiterErr, "a live waiter must not inherit the owner's cancellation")
		assert.Equal(t, int32(2), calls.Load())
		assert.NoError(t, c.Close())
	})
	t.Run("no retry when the loadFunc fails with its own internal timeout", func(t *testing.T) {
		// A loadFunc-internal deadline (e.g. a transport dial timeout)
		// returns a context error while the load's own ctx is alive —
		// that is a verdict about the resource, not a killed load, and
		// must surface immediately instead of multiplying dial attempts.
		var calls atomic.Int32
		c := New(func(ctx context.Context, id string) (Object, error) {
			calls.Add(1)
			return nil, fmt.Errorf("dial: %w", context.DeadlineExceeded)
		})
		_, err := c.Get(context.Background(), "id")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, int32(1), calls.Load())
		assert.NoError(t, c.Close())
	})
	t.Run("retry is bounded under repeated aborts", func(t *testing.T) {
		// White-box: a pre-poisoned entry (failed, aborted, never removed
		// from the map — unlike a real load) makes every retry observe the
		// same aborted result, so Get must give up at maxLoadRetries
		// instead of spinning.
		c := New(func(ctx context.Context, id string) (Object, error) {
			return &testObject{name: id}, nil
		}).(*oCache)
		e := newEntry("id", nil, entryStateLoading)
		e.loadErr = context.Canceled
		e.loadAborted = true
		close(e.load)
		c.mu.Lock()
		c.data["id"] = e
		c.mu.Unlock()
		_, err := c.Get(context.Background(), "id")
		require.ErrorIs(t, err, context.Canceled)
		c.mu.Lock()
		delete(c.data, "id")
		c.mu.Unlock()
		assert.NoError(t, c.Close())
	})
	t.Run("no retry when the caller's own ctx is done", func(t *testing.T) {
		var calls atomic.Int32
		c := New(func(ctx context.Context, id string) (Object, error) {
			calls.Add(1)
			return nil, ctx.Err()
		})
		cctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.Get(cctx, "id")
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(1), calls.Load())
		assert.NoError(t, c.Close())
	})
}

func TestOCache_GC(t *testing.T) {
	t.Run("test gc expired object", func(t *testing.T) {
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, true, nil), nil
		}, WithTTL(time.Millisecond*10))
		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		c.GC()
		assert.Equal(t, 1, c.Len())
		time.Sleep(time.Millisecond * 20)
		c.GC()
		assert.Equal(t, 0, c.Len())
	})
	t.Run("test gc tryClose true, close before get", func(t *testing.T) {
		closeCh := make(chan struct{})
		getCh := make(chan struct{})

		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, true, closeCh), nil
		}, WithTTL(time.Millisecond*10))
		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		// making ttl pass
		time.Sleep(time.Millisecond * 20)
		// first gc will be run after 20 secs, so calling it manually
		go c.GC()
		// waiting until all objects are marked as closing
		time.Sleep(time.Millisecond * 20)
		var events []string
		go func() {
			// defer + assert (not require): a failed assertion must still
			// close getCh, or the subtest deadlocks instead of failing
			defer close(getCh)
			_, err := c.Get(context.TODO(), "id")
			assert.NoError(t, err)
			assert.NotNil(t, val)
			events = append(events, "get")
		}()
		// sleeping to make sure that Get is called
		time.Sleep(time.Millisecond * 20)
		events = append(events, "close")
		close(closeCh)

		<-getCh
		require.Equal(t, []string{"close", "get"}, events)
	})
	t.Run("test gc tryClose false, many parallel get", func(t *testing.T) {
		timesCalled := &atomic.Int32{}
		obj := NewTestObject("id", false, nil)
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			timesCalled.Add(1)
			return obj, nil
		}, WithTTL(0))

		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		begin := make(chan struct{})
		wg := sync.WaitGroup{}
		once := sync.Once{}

		wg.Add(1)
		go func() {
			<-begin
			c.GC()
			wg.Done()
		}()
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(i int) {
				once.Do(func() {
					close(begin)
				})
				if i%2 != 0 {
					time.Sleep(time.Millisecond)
				}
				_, err := c.Get(context.TODO(), "id")
				require.NoError(t, err)
				wg.Done()
			}(i)
		}
		require.NoError(t, err)
		wg.Wait()
		require.Equal(t, timesCalled.Load(), int32(1))
		require.True(t, obj.tryCloseCalled)
	})
	t.Run("test gc tryClose different, many objects", func(t *testing.T) {
		tryCloseIds := make(map[string]bool)
		called := make(map[string]int)
		max := 1000
		getId := func(i int) string {
			return fmt.Sprintf("id%d", i)
		}
		for i := 0; i < max; i++ {
			if i%2 == 1 {
				tryCloseIds[getId(i)] = true
			} else {
				tryCloseIds[getId(i)] = false
			}
		}
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			called[id] = called[id] + 1
			return NewTestObject(id, tryCloseIds[id], nil), nil
		}, WithTTL(time.Millisecond*10))

		for i := 0; i < max; i++ {
			val, err := c.Get(context.TODO(), getId(i))
			require.NoError(t, err)
			require.NotNil(t, val)
		}
		assert.Equal(t, max, c.Len())
		time.Sleep(time.Millisecond * 20)
		c.GC()
		for i := 0; i < max; i++ {
			val, err := c.Get(context.TODO(), getId(i))
			require.NoError(t, err)
			require.NotNil(t, val)
		}
		for i := 0; i < max; i++ {
			val, err := c.Get(context.TODO(), getId(i))
			require.NoError(t, err)
			require.NotNil(t, val)
			require.Equal(t, called[getId(i)], i%2+1)
		}
	})
}

func Test_OCache_Remove(t *testing.T) {
	t.Run("remove simple", func(t *testing.T) {
		closeCh := make(chan struct{})
		getCh := make(chan struct{})
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, false, closeCh), nil
		}, WithTTL(time.Millisecond*10))

		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		// removing the object, so we will wait on closing
		removeDone := make(chan struct{})
		go func() {
			defer close(removeDone)
			_, err := c.Remove(ctx, "id")
			assert.NoError(t, err)
		}()
		time.Sleep(time.Millisecond * 20)

		var events []string
		go func() {
			// defer + assert (not require): a failed assertion must still
			// close getCh, or the subtest deadlocks instead of failing
			defer close(getCh)
			_, err := c.Get(context.TODO(), "id")
			assert.NoError(t, err)
			assert.NotNil(t, val)
			events = append(events, "get")
		}()
		// sleeping to make sure that Get is called
		time.Sleep(time.Millisecond * 20)
		events = append(events, "close")
		close(closeCh)

		<-getCh
		<-removeDone
		require.Equal(t, []string{"close", "get"}, events)
	})
	t.Run("tryRemove simple", func(t *testing.T) {
		closeCh := make(chan struct{})
		getCh := make(chan struct{})
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, true, closeCh), nil
		}, WithTTL(time.Millisecond*10))

		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		// try removing the object, so we will wait on closing. Len is not
		// asserted here: the parked Get re-creates the entry as soon as the
		// removal completes, so 0 is only ever a transient state.
		tryRemoveDone := make(chan struct{})
		go func() {
			defer close(tryRemoveDone)
			ok, err := c.TryRemove("id")
			assert.True(t, ok)
			assert.NoError(t, err)
		}()
		time.Sleep(time.Millisecond * 20)

		var events []string
		go func() {
			defer close(getCh)
			_, err := c.Get(context.TODO(), "id")
			assert.Equal(t, 1, c.Len())
			assert.NoError(t, err)
			assert.NotNil(t, val)
			events = append(events, "get")
		}()
		// sleeping to make sure that Get is called
		time.Sleep(time.Millisecond * 20)
		events = append(events, "close")
		close(closeCh)

		<-getCh
		<-tryRemoveDone
		require.Equal(t, []string{"close", "get"}, events)
	})
	t.Run("tryRemove simple - can't be removed", func(t *testing.T) {
		closeCh := make(chan struct{})
		getCh := make(chan struct{})
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, false, closeCh), nil
		}, WithTTL(time.Millisecond*10))

		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		// try removing the object, so we will wait on closing
		tryRemoveDone := make(chan struct{})
		go func() {
			defer close(tryRemoveDone)
			ok, err := c.TryRemove("id")
			assert.False(t, ok)
			assert.NoError(t, err)
		}()
		time.Sleep(time.Millisecond * 20)

		var events []string
		go func() {
			defer close(getCh)
			_, err := c.Get(context.TODO(), "id")
			assert.Equal(t, 1, c.Len())
			assert.NoError(t, err)
			assert.NotNil(t, val)
			events = append(events, "get")
		}()
		// sleeping to make sure that Get is called
		time.Sleep(time.Millisecond * 20)
		events = append(events, "close")
		close(closeCh)

		<-getCh
		<-tryRemoveDone
		require.Equal(t, 1, c.Len())
		require.Equal(t, []string{"close", "get"}, events)
	})
	t.Run("test remove while gc, tryClose false", func(t *testing.T) {
		closeCh := make(chan struct{})
		removeCh := make(chan struct{})

		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, false, closeCh), nil
		}, WithTTL(time.Millisecond*10))
		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		time.Sleep(time.Millisecond * 20)
		go c.GC()
		time.Sleep(time.Millisecond * 20)
		var events []string
		go func() {
			defer close(removeCh)
			ok, err := c.Remove(ctx, "id")
			assert.NoError(t, err)
			assert.True(t, ok)
			events = append(events, "remove")
		}()
		time.Sleep(time.Millisecond * 20)
		events = append(events, "close")
		close(closeCh)

		<-removeCh
		require.Equal(t, []string{"close", "remove"}, events)
	})
	t.Run("test remove while gc, tryClose true", func(t *testing.T) {
		closeCh := make(chan struct{})
		removeCh := make(chan struct{})

		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, true, closeCh), nil
		}, WithTTL(time.Millisecond*10))
		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		time.Sleep(time.Millisecond * 20)
		go c.GC()
		time.Sleep(time.Millisecond * 20)
		var events []string
		go func() {
			defer close(removeCh)
			ok, err := c.Remove(ctx, "id")
			assert.NoError(t, err)
			assert.False(t, ok)
			events = append(events, "remove")
		}()
		time.Sleep(time.Millisecond * 20)
		events = append(events, "close")
		close(closeCh)

		<-removeCh
		require.Equal(t, []string{"close", "remove"}, events)
	})
	t.Run("test gc while remove, tryClose true", func(t *testing.T) {
		closeCh := make(chan struct{})
		removeCh := make(chan struct{})

		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, true, closeCh), nil
		}, WithTTL(time.Millisecond*10))
		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		go func() {
			defer close(removeCh)
			ok, err := c.Remove(ctx, "id")
			assert.NoError(t, err)
			assert.True(t, ok)
		}()
		time.Sleep(20 * time.Millisecond)
		c.GC()
		close(closeCh)
		<-removeCh
	})
}

func TestOCacheCancelWhenRemove(t *testing.T) {
	c := New(func(ctx context.Context, id string) (value Object, err error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, WithTTL(time.Millisecond*10))
	stopLoad := make(chan struct{})
	var err error
	go func() {
		_, err = c.Get(context.TODO(), "id")
		stopLoad <- struct{}{}
	}()
	time.Sleep(time.Millisecond * 10)
	c.Close()
	<-stopLoad
	// Close cancels the in-flight load; the caller's retry then observes
	// the closed cache and reports ErrClosed — the actual reason — rather
	// than the load's raw context.Canceled.
	require.Equal(t, ErrClosed, err)
}

func TestOCacheFuzzy(t *testing.T) {
	t.Run("test many objects gc, get and remove simultaneously, close after", func(t *testing.T) {
		tryCloseIds := make(map[string]bool)
		max := 2000
		getId := func(i int) string {
			return fmt.Sprintf("id%d", i)
		}
		for i := 0; i < max; i++ {
			if i%2 == 1 {
				tryCloseIds[getId(i)] = true
			} else {
				tryCloseIds[getId(i)] = false
			}
		}
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, tryCloseIds[id], nil), nil
		}, WithTTL(time.Nanosecond))

		stopGC := make(chan struct{})
		wg := sync.WaitGroup{}
		go func() {
			for {
				select {
				case <-stopGC:
					return
				default:
					c.GC()
				}
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				for i := 0; i < max; i++ {
					val, err := c.Get(context.TODO(), getId(i))
					require.NoError(t, err)
					require.NotNil(t, val)
				}
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				for i := 0; i < max; i++ {
					c.Remove(ctx, getId(i))
				}
			}
		}()
		wg.Wait()
		close(stopGC)
		err := c.Close()
		require.NoError(t, err)
		require.Equal(t, 0, c.Len())
	})
	t.Run("test many objects gc, get, remove and close simultaneously", func(t *testing.T) {
		tryCloseIds := make(map[string]bool)
		max := 2000
		getId := func(i int) string {
			return fmt.Sprintf("id%d", i)
		}
		for i := 0; i < max; i++ {
			if i%2 == 1 {
				tryCloseIds[getId(i)] = true
			} else {
				tryCloseIds[getId(i)] = false
			}
		}
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, tryCloseIds[id], nil), nil
		}, WithTTL(time.Nanosecond))

		stopGC := make(chan struct{})
		defer close(stopGC)
		go func() {
			for {
				select {
				case <-stopGC:
					return
				default:
					c.GC()
				}
			}
		}()
		go func() {
			for j := 0; j < 10; j++ {
				for i := 0; i < max; i++ {
					val, err := c.Get(context.TODO(), getId(i))
					if err == ErrClosed {
						return
					}
					assert.NoError(t, err)
					assert.NotNil(t, val)
				}
			}
		}()
		go func() {
			for j := 0; j < 10; j++ {
				for i := 0; i < max; i++ {
					c.Remove(ctx, getId(i))
				}
			}
		}()
		time.Sleep(time.Millisecond)
		err := c.Close()
		require.NoError(t, err)
		require.Equal(t, 0, c.Len())
	})
}

func TestOCache_RemoveSame(t *testing.T) {
	newCache := func() OCache {
		return New(func(ctx context.Context, id string) (Object, error) {
			return nil, ErrNotExists
		})
	}

	t.Run("removes matching value", func(t *testing.T) {
		c := newCache()
		obj := NewTestObject("1", false, nil)
		require.NoError(t, c.Add("1", obj))
		ok, err := c.RemoveSame(ctx, "1", obj)
		require.NoError(t, err)
		require.True(t, ok)
		require.True(t, obj.closeCalled)
		require.Equal(t, 0, c.Len())
	})

	t.Run("keeps replacement under same id", func(t *testing.T) {
		c := newCache()
		old := NewTestObject("old", false, nil)
		repl := NewTestObject("repl", false, nil)
		require.NoError(t, c.Add("1", old))
		// old gets replaced by repl under the same id
		_, err := c.Remove(ctx, "1")
		require.NoError(t, err)
		require.NoError(t, c.Add("1", repl))
		// a stale RemoveSame holding the old value must not touch repl
		ok, err := c.RemoveSame(ctx, "1", old)
		require.ErrorIs(t, err, ErrNotExists)
		require.False(t, ok)
		require.False(t, repl.closeCalled, "replacement must not be closed")
		require.Equal(t, 1, c.Len())
		pk, err := c.Pick(ctx, "1")
		require.NoError(t, err)
		require.Equal(t, repl, pk)
	})

	t.Run("missing id", func(t *testing.T) {
		c := newCache()
		obj := NewTestObject("1", false, nil)
		ok, err := c.RemoveSame(ctx, "1", obj)
		require.ErrorIs(t, err, ErrNotExists)
		require.False(t, ok)
	})
}

// busyRevertObject deterministically drives the
// active -> closing -> active "busy revert" path: TryClose blocks until
// release is closed, then reports "not closed" (busy) so the cache reverts
// the entry to active via setActive(true). Close is benign/idempotent on
// purpose, so a double removal surfaces as the close-channel double-close
// (the production panic) rather than masking it.
type busyRevertObject struct {
	release chan struct{}
}

func (o *busyRevertObject) Close() (err error) {
	// Hold the entry between setClosing and setClosed long enough for a
	// second parked remover to observe it mid-close and overwrite e.close.
	time.Sleep(2 * time.Millisecond)
	return nil
}

func (o *busyRevertObject) TryClose(objectTTL time.Duration) (res bool, err error) {
	<-o.release
	return false, nil
}

// TestOCache_RemoveBusyRevertDoubleClose reproduces a panic that crashes the
// process on laptop wake (GO-7332): when a busy TryRemove/GC reverts an entry
// back to active while two or more Remove callers are parked on its close
// channel, every woken remover re-enters the closing path. The second one
// overwrites e.close, and both call setClosed -> close(e.close) on the same
// channel: "panic: close of closed channel".
//
// Covered combinations: the revert is driven by both TryRemove and GC (the two
// setActive(true) call sites), against 2 and 3 parked removers (3 removers is
// also what surfaces the pre-fix cache-wide deadlock when setClosed panics
// while holding c.mu).
func TestOCache_RemoveBusyRevertDoubleClose(t *testing.T) {
	reverters := []struct {
		name string
		ttl  time.Duration // 0 makes the Added entry eligible for GC immediately
		fn   func(c *oCache)
	}{
		{"TryRemove", time.Minute, func(c *oCache) { _, _ = c.TryRemove("id") }},
		{"GC", 0, func(c *oCache) { c.GC() }},
	}
	for _, rev := range reverters {
		for _, removers := range []int{2, 3} {
			rev, removers := rev, removers
			t.Run(fmt.Sprintf("%s/%dremovers", rev.name, removers), func(t *testing.T) {
				obj := &busyRevertObject{release: make(chan struct{})}
				c := New(
					func(ctx context.Context, id string) (Object, error) { return obj, nil },
					WithGCPeriod(0), // no background GC; we drive the revert manually
					WithTTL(rev.ttl),
				).(*oCache)
				require.NoError(t, c.Add("id", obj))

				// 1) busy reverter: marks the entry closing (creates the close
				//    channel), then blocks in TryClose holding it in closing.
				revDone := make(chan struct{})
				go func() {
					defer close(revDone)
					rev.fn(c)
				}()

				// wait until the entry is actually in the closing state
				require.Eventually(t, func() bool {
					st, ok := entryStateOf(c, "id")
					return ok && st == entryStateClosing
				}, time.Second, time.Millisecond)

				// 2) removers park on the closing channel.
				var panicMsg atomic.Value
				var wg sync.WaitGroup
				wg.Add(removers)
				for i := 0; i < removers; i++ {
					go func() {
						defer wg.Done()
						defer func() {
							if r := recover(); r != nil {
								panicMsg.Store(fmt.Sprint(r))
							}
						}()
						_, _ = c.Remove(ctx, "id")
					}()
				}
				time.Sleep(20 * time.Millisecond) // let removers park on <-e.close

				// 3) release TryClose -> "busy" -> setActive(true) closes the
				//    channel and reverts to active, waking the removers.
				close(obj.release)

				<-revDone
				wg.Wait()

				if msg := panicMsg.Load(); msg != nil {
					t.Fatalf("Remove double-closed the entry's close channel: panic %q", msg)
				}
			})
		}
	}
}

// An entry another closer already owns must not stall cache close: that closer
// can be a gc sitting in TryClose against an unresponsive peer.
func TestOCache_CloseBoundedByOtherCloser(t *testing.T) {
	c := New(func(ctx context.Context, id string) (Object, error) {
		return NewTestObject(id, true, nil), nil
	})
	oc := c.(*oCache)
	oc.closeTimeout = 20 * time.Millisecond
	_, err := c.Get(ctx, "id")
	require.NoError(t, err)

	oc.mu.Lock()
	e := oc.data["id"]
	oc.mu.Unlock()
	_, curState, err := e.setClosing(ctx, false)
	require.NoError(t, err)
	require.Equal(t, entryState(entryStateClosing), curState)

	done := make(chan struct{})
	go func() {
		_ = c.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "Close blocked behind an entry owned by another closer")
	}
}

// The close deadline is spent only on entries another closer holds: healthy
// entries must still be closed after it has expired.
func TestOCache_CloseClosesHealthyEntriesAfterDeadline(t *testing.T) {
	var objects []*testObject
	c := New(func(ctx context.Context, id string) (Object, error) {
		o := NewTestObject(id, true, nil)
		objects = append(objects, o)
		return o, nil
	})
	oc := c.(*oCache)
	oc.closeTimeout = time.Millisecond
	for i := 0; i < 50; i++ {
		_, err := c.Get(ctx, fmt.Sprint(i))
		require.NoError(t, err)
	}
	// one entry is held by another closer and will eat the whole deadline
	oc.mu.Lock()
	held := oc.data["0"]
	oc.mu.Unlock()
	_, _, err := held.setClosing(ctx, false)
	require.NoError(t, err)

	require.NoError(t, c.Close())
	var notClosed []string
	for _, o := range objects {
		if o.name != "0" && !o.closeCalled {
			notClosed = append(notClosed, o.name)
		}
	}
	require.Empty(t, notClosed, "entries nobody else holds must still be closed after the deadline")
}

// entryStateOf reports the state of the entry stored under id, if any.
func entryStateOf(c *oCache, id string) (state entryState, ok bool) {
	c.mu.Lock()
	e, ok := c.data[id]
	c.mu.Unlock()
	if !ok {
		return
	}
	e.mx.Lock()
	defer e.mx.Unlock()
	return e.state, true
}

// newLoadingCache returns a cache with one Get in flight on "id": the loadFunc
// has signalled it is running and blocks until release is closed, then
// publishes obj. getDone receives the Get's result.
func newLoadingCache(opts ...Option) (c *oCache, obj *testObject, release chan struct{}, getDone chan error) {
	loading := make(chan struct{})
	release = make(chan struct{})
	obj = NewTestObject("id", true, nil)
	var loadingOnce sync.Once // a retried load must not close(loading) twice
	c = New(func(loadCtx context.Context, id string) (Object, error) {
		loadingOnce.Do(func() { close(loading) })
		<-release
		return obj, nil
	}, append([]Option{WithGCPeriod(0)}, opts...)...).(*oCache)
	getDone = make(chan error, 1)
	go func() {
		v, err := c.Get(ctx, "id")
		if err == nil && v != obj {
			err = fmt.Errorf("Get returned %v, want the loaded object", v)
		}
		getDone <- err
	}()
	<-loading
	return
}

// awaitClose fails the test when c.Close does not return in time.
func awaitClose(t *testing.T, c OCache, failMsg string) {
	t.Helper()
	closeErr := make(chan error, 1)
	go func() { closeErr <- c.Close() }()
	select {
	case err := <-closeErr:
		require.NoError(t, err)
	case <-time.After(time.Second * 2):
		require.Fail(t, failMsg)
	}
}

// A loading entry has a nil value and an open load channel: oCache.load alone
// publishes e.value and closes e.load. The try-close paths (TryRemove, GC)
// must refuse such an entry - setClosing refuses the loading->closing
// transition - and leave the in-flight load untouched. The subtests pin the
// three ways touching one goes wrong: a nil-value TryClose panic, a Get
// stranded in waitClose behind a close channel the load never closes, and an
// unsynchronised read of e.value racing its publication.
func TestOCache_TryRemoveWhileLoading(t *testing.T) {
	t.Run("refuses a loading entry and leaves the load running", func(t *testing.T) {
		c, _, release, getDone := newLoadingCache()

		state, ok := entryStateOf(c, "id")
		require.True(t, ok)
		require.Equal(t, entryState(entryStateLoading), state)

		var (
			removed bool
			err     error
		)
		require.NotPanics(t, func() { removed, err = c.TryRemove("id") }, "TryRemove on a loading entry")
		require.NoError(t, err)
		require.False(t, removed, "a loading entry has no value to close")

		state, ok = entryStateOf(c, "id")
		require.True(t, ok, "TryRemove must not drop an entry it refused")
		require.Equal(t, entryState(entryStateLoading), state,
			"the entry must be left loading, not parked in closing")

		close(release)
		select {
		case err := <-getDone:
			require.NoError(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "the in-flight load never completed")
		}
		require.NoError(t, c.Close())
	})

	t.Run("a Get arriving after the refused TryRemove is not stranded", func(t *testing.T) {
		// A Get first calls waitClose, so an entry wrongly left in
		// entryStateClosing parks it on a close channel that the completing
		// load never closes.
		c, _, release, firstGet := newLoadingCache()

		require.NotPanics(t, func() { _, _ = c.TryRemove("id") }, "TryRemove on a loading entry")
		state, ok := entryStateOf(c, "id")
		require.True(t, ok)
		require.Equal(t, entryState(entryStateLoading), state,
			"a refused TryRemove must leave the entry loading; closing strands the late Get")

		lateGet := make(chan error, 1)
		go func() {
			v, err := c.Get(ctx, "id")
			if err == nil && v == nil {
				err = errors.New("nil value")
			}
			lateGet <- err
		}()
		// best effort at letting the late Get park before the load completes;
		// the state assertion above is the deterministic regression pin
		time.Sleep(time.Millisecond * 10)
		close(release)

		for _, res := range []chan error{firstGet, lateGet} {
			select {
			case err := <-res:
				require.NoError(t, err)
			case <-time.After(time.Second):
				require.Fail(t, "a caller is stuck on an entry TryRemove parked in closing")
			}
		}
		require.NoError(t, c.Close())
	})

	t.Run("the try-close path refuses a loading entry", func(t *testing.T) {
		// GC pre-filters on e.isActive(), and nothing moves an entry back to
		// loading (newEntry is the only writer of entryStateLoading, and a
		// fresh entry replaces a removed one), so GC cannot reach a loading
		// value through its public path — pin the shared tryCloseEntry helper
		// directly instead, then check the GC pass as a whole stays inert.
		c, _, release, getDone := newLoadingCache(WithTTL(0))

		c.mu.Lock()
		e := c.data["id"]
		c.mu.Unlock()
		var (
			closed bool
			err    error
		)
		require.NotPanics(t, func() { closed, err = c.tryCloseEntry(e) }, "tryCloseEntry on a loading entry")
		require.False(t, closed, "a loading entry has no value to close")
		require.NoError(t, err)
		require.NotPanics(t, func() { c.GC() }, "GC with a loading entry in the map")

		state, ok := entryStateOf(c, "id")
		require.True(t, ok)
		require.Equal(t, entryState(entryStateLoading), state)

		close(release)
		require.NoError(t, <-getDone)
		require.NoError(t, c.Close())
	})

	t.Run("Close still closes an entry that was loading", func(t *testing.T) {
		// Close cancels every in-flight load and waits it out. This loadFunc
		// still produces a value on cancellation; whichever side wins the race
		// - Close's own pass, or the load's closed-cache branch - the value
		// must end up closed and the entry gone.
		var (
			loading = make(chan struct{})
			obj     = NewTestObject("id", true, nil)
		)
		c := New(func(loadCtx context.Context, id string) (Object, error) {
			close(loading)
			<-loadCtx.Done()
			return obj, nil
		}, WithGCPeriod(0)).(*oCache)

		go func() { _, _ = c.Get(ctx, "id") }()
		<-loading

		awaitClose(t, c, "Close blocked on an entry that was loading")
		require.True(t, obj.closeCalled, "Close must close an entry that was still loading")
		require.Equal(t, 0, c.Len())
	})

	t.Run("Close does not hang on a load it aborted", func(t *testing.T) {
		var loading = make(chan struct{})
		c := New(func(loadCtx context.Context, id string) (Object, error) {
			close(loading)
			<-loadCtx.Done()
			return nil, loadCtx.Err()
		}, WithGCPeriod(0)).(*oCache)

		go func() { _, _ = c.Get(ctx, "id") }()
		<-loading

		awaitClose(t, c, "Close blocked on a load it aborted")
		require.Equal(t, 0, c.Len())
	})

	t.Run("TryRemove racing the value publication", func(t *testing.T) {
		// oCache.load writes e.value under c.mu; TryRemove must not read it
		// unsynchronised. The release goroutine is not waited for, so
		// TryRemove observes both the loading and the just-published states
		// across the 100 iterations, and -race can catch an unguarded pair.
		for i := 0; i < 100; i++ {
			c, _, release, getDone := newLoadingCache()

			go func() { close(release) }() // the load publishes e.value ...
			if i%2 == 1 {
				// without a yield TryRemove always observes the loading state;
				// yielding lets half the iterations meet the published state
				runtime.Gosched()
			}
			require.NotPanics(t, func() { _, _ = c.TryRemove("id") }, "TryRemove racing the load") // ... while TryRemove reads it
			require.NoError(t, <-getDone)
			require.NoError(t, c.Close())
		}
	})
}

// tryCloseResultObject reports a fixed TryClose verdict.
type tryCloseResultObject struct {
	res         bool
	err         error
	closeCalled bool
}

func (o *tryCloseResultObject) Close() error {
	if o.closeCalled {
		panic("close called twice")
	}
	o.closeCalled = true
	return nil
}

func (o *tryCloseResultObject) TryClose(objectTTL time.Duration) (bool, error) { return o.res, o.err }

// A TryClose error must not abandon the acquired closing transition: a value
// that declined to close stays usable, a value that closed leaves the map.
func TestOCache_TryRemoveTryCloseError(t *testing.T) {
	t.Run("declined with an error: entry restored, id stays usable", func(t *testing.T) {
		obj := &tryCloseResultObject{err: errors.New("transient close error")}
		c := New(func(loadCtx context.Context, id string) (Object, error) {
			return obj, nil
		}, WithGCPeriod(0)).(*oCache)
		require.NoError(t, c.Add("id", obj))

		ok, err := c.TryRemove("id")
		require.False(t, ok)
		require.Error(t, err)

		state, found := entryStateOf(c, "id")
		require.True(t, found, "the entry must survive a declined TryClose")
		require.Equal(t, entryState(entryStateActive), state,
			"a declined entry is restored to active, not parked in closing")

		getCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		v, err := c.Get(getCtx, "id")
		require.NoError(t, err, "the id must stay usable after a TryClose error")
		require.Same(t, obj, v)
		require.NoError(t, c.Close())
	})

	t.Run("closed with an error: entry removed", func(t *testing.T) {
		obj := &tryCloseResultObject{res: true, err: errors.New("closed with error")}
		c := New(func(loadCtx context.Context, id string) (Object, error) {
			return obj, nil
		}, WithGCPeriod(0)).(*oCache)
		require.NoError(t, c.Add("id", obj))

		ok, err := c.TryRemove("id")
		require.True(t, ok)
		require.Error(t, err)
		require.Equal(t, 0, c.Len(), "a closed value must not stay in the map")
		require.False(t, obj.closeCalled, "TryClose already closed the value; Close must not run on top")
		require.NoError(t, c.Close())
	})
}

// RemoveSame matches by pointer identity; nil identifies nothing, in
// particular not a mid-load entry whose value is still nil.
func TestOCache_RemoveSameNilValue(t *testing.T) {
	c, obj, release, getDone := newLoadingCache()

	rmCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	ok, err := c.RemoveSame(rmCtx, "id", nil)
	require.False(t, ok)
	require.ErrorIs(t, err, ErrNotExists)

	close(release)
	require.NoError(t, <-getDone)
	require.False(t, obj.closeCalled, "RemoveSame(nil) must not close the value a load publishes")
	require.NoError(t, c.Close())
}

func TestOCache_AddAfterClose(t *testing.T) {
	c := New(func(loadCtx context.Context, id string) (Object, error) {
		return NewTestObject(id, true, nil), nil
	})
	require.NoError(t, c.Close())
	require.ErrorIs(t, c.Add("id", NewTestObject("id", true, nil)), ErrClosed)
	require.Equal(t, 0, c.Len())
}

// A loadFunc that ignores cancellation must not wedge Close, and the value it
// publishes after Close gave up must not leak: the load's closed-cache branch
// closes it and drops the entry, and waiters get ErrClosed.
func TestOCache_CloseBoundedByWedgedLoad(t *testing.T) {
	loading := make(chan struct{})
	release := make(chan struct{})
	obj := NewTestObject("id", true, nil)
	c := New(func(loadCtx context.Context, id string) (Object, error) {
		close(loading)
		<-release // deliberately ignores loadCtx
		return obj, nil
	}, WithGCPeriod(0), WithCloseTimeout(50*time.Millisecond)).(*oCache)

	getDone := make(chan error, 1)
	go func() {
		_, err := c.Get(ctx, "id")
		getDone <- err
	}()
	<-loading

	awaitClose(t, c, "Close hung behind a load that ignores cancellation")

	// the forfeited load completes after Close returned
	close(release)
	require.ErrorIs(t, <-getDone, ErrClosed, "a waiter must not receive a value from a closed cache")
	// the Get goroutine is the loader: it ran value.Close inside c.load before
	// returning, so these reads are ordered after it
	require.True(t, obj.closeCalled, "the late-published value must be closed by the load itself")
	require.Equal(t, 0, c.Len(), "the forfeited entry must not stay in the map")
}

// Add(nil) must be refused: every close path dereferences the value, and the
// GC one runs on the ticker goroutine where a panic kills the process. A
// typed-nil pointer is the same hazard wrapped in a non-nil interface.
func TestOCache_AddNilValue(t *testing.T) {
	c := New(func(loadCtx context.Context, id string) (Object, error) {
		return NewTestObject(id, true, nil), nil
	})
	require.ErrorIs(t, c.Add("id", nil), ErrNilValue)
	require.ErrorIs(t, c.Add("id", (*testObject)(nil)), ErrNilValue)
	require.Equal(t, 0, c.Len())
	require.NoError(t, c.Close())
}

// A closer that outruns Close's deadline and then declines must not restore
// its entry into the closed cache — a live value would survive shutdown with
// no remaining closer. The decline escalates to a real close instead.
func TestOCache_CloseBoundedByBusyCloser(t *testing.T) {
	block := make(chan struct{})
	obj := NewTestObject("id", false, block) // TryClose blocks on block, then declines
	c := New(func(loadCtx context.Context, id string) (Object, error) {
		return obj, nil
	}, WithGCPeriod(0), WithCloseTimeout(50*time.Millisecond)).(*oCache)
	require.NoError(t, c.Add("id", obj))

	tryRemoveDone := make(chan struct{})
	go func() {
		defer close(tryRemoveDone)
		ok, err := c.TryRemove("id")
		assert.True(t, ok, "the decline into a closed cache must escalate to a removal")
		assert.NoError(t, err)
	}()
	require.Eventually(t, func() bool {
		st, ok := entryStateOf(c, "id")
		return ok && st == entryStateClosing
	}, time.Second, time.Millisecond)

	awaitClose(t, c, "Close hung behind a busy closer")

	// the closer wakes after Close gave up on it and declines
	close(block)
	<-tryRemoveDone
	require.True(t, obj.closeCalled, "the declined value must be closed, not resurrected")
	require.Equal(t, 0, c.Len(), "the entry must not survive in the closed cache")
}

// A panicking loadFunc must not wedge its id: the entry is dropped, waiters
// get an error rather than (nil, nil), and the panic reaches the loading
// caller.
func TestOCache_LoadFuncPanic(t *testing.T) {
	calls := 0
	c := New(func(loadCtx context.Context, id string) (Object, error) {
		calls++
		if calls == 1 {
			panic("boom")
		}
		return NewTestObject(id, true, nil), nil
	}, WithGCPeriod(0)).(*oCache)

	func() {
		defer func() { require.NotNil(t, recover(), "the loadFunc panic must propagate") }()
		_, _ = c.Get(ctx, "id")
	}()

	v, err := c.Get(ctx, "id")
	require.NoError(t, err, "a fresh Get must run a fresh load, not observe the panicked entry")
	require.NotNil(t, v)
	require.Equal(t, 2, calls)
	require.NoError(t, c.Close())
}

func TestOCache_PickAfterClose(t *testing.T) {
	c := New(func(loadCtx context.Context, id string) (Object, error) {
		return NewTestObject(id, true, nil), nil
	})
	_, err := c.Get(ctx, "id")
	require.NoError(t, err)
	require.NoError(t, c.Close())
	_, err = c.Pick(ctx, "id")
	require.ErrorIs(t, err, ErrClosed)
}
