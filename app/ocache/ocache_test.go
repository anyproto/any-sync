package ocache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func (t *testObject) TryClose() (res bool, err error) {
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
			_, err := c.Get(context.TODO(), "id")
			require.NoError(t, err)
			require.NotNil(t, val)
			events = append(events, "get")
			close(getCh)
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
		}, WithTTL(time.Millisecond*10))

		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		time.Sleep(time.Millisecond * 20)
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
		go func() {
			_, err := c.Remove("id")
			require.NoError(t, err)
		}()
		time.Sleep(time.Millisecond * 20)

		var events []string
		go func() {
			_, err := c.Get(context.TODO(), "id")
			require.NoError(t, err)
			require.NotNil(t, val)
			events = append(events, "get")
			close(getCh)
		}()
		// sleeping to make sure that Get is called
		time.Sleep(time.Millisecond * 20)
		events = append(events, "close")
		close(closeCh)

		<-getCh
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
			ok, err := c.Remove("id")
			require.NoError(t, err)
			require.True(t, ok)
			events = append(events, "remove")
			close(removeCh)
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
			ok, err := c.Remove("id")
			require.NoError(t, err)
			require.False(t, ok)
			events = append(events, "remove")
			close(removeCh)
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
			ok, err := c.Remove("id")
			require.NoError(t, err)
			require.True(t, ok)
			close(removeCh)
		}()
		time.Sleep(20 * time.Millisecond)
		c.GC()
		close(closeCh)
		<-removeCh
	})
}

func TestOCacheFuzzy(t *testing.T) {
	t.Run("test many objects gc, get and remove simultaneously, close after", func(t *testing.T) {
		tryCloseIds := make(map[string]bool)
		called := make(map[string]int)
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
			called[id] = called[id] + 1
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
					c.Remove(getId(i))
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
		called := make(map[string]int)
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
			called[id] = called[id] + 1
			return NewTestObject(id, tryCloseIds[id], nil), nil
		}, WithTTL(time.Nanosecond))

		go func() {
			for {
				c.GC()
			}
		}()
		go func() {
			for j := 0; j < 10; j++ {
				for i := 0; i < max; i++ {
					val, err := c.Get(context.TODO(), getId(i))
					if err == ErrClosed {
						return
					}
					require.NoError(t, err)
					require.NotNil(t, val)
				}
			}
		}()
		go func() {
			for j := 0; j < 10; j++ {
				for i := 0; i < max; i++ {
					c.Remove(getId(i))
				}
			}
		}()
		time.Sleep(time.Millisecond)
		err := c.Close()
		require.NoError(t, err)
		require.Equal(t, 0, c.Len())
	})
}
