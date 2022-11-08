package ocache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testObject struct {
	name     string
	closeErr error
	closeCh  chan struct{}
}

func NewTestObject(name string, closeCh chan struct{}) *testObject {
	return &testObject{
		name:    name,
		closeCh: closeCh,
	}
}

func (t *testObject) Close() (err error) {
	if t.closeCh != nil {
		<-t.closeCh
	}
	return t.closeErr
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
	t.Run("test without close wait", func(t *testing.T) {
		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return &testObject{name: id}, nil
		}, WithTTL(time.Millisecond*10), WithRefCounter(true))
		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		c.GC()
		assert.Equal(t, 1, c.Len())
		time.Sleep(time.Millisecond * 30)
		c.GC()
		assert.Equal(t, 1, c.Len())
		assert.True(t, c.Release("id"))
		c.GC()
		assert.Equal(t, 0, c.Len())
		assert.False(t, c.Release("id"))
	})
	t.Run("test with close wait", func(t *testing.T) {
		closeCh := make(chan struct{})
		getCh := make(chan struct{})

		c := New(func(ctx context.Context, id string) (value Object, err error) {
			return NewTestObject(id, closeCh), nil
		}, WithTTL(time.Millisecond*10), WithRefCounter(true))
		val, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		assert.Equal(t, 1, c.Len())
		assert.True(t, c.Release("id"))
		// making ttl pass
		time.Sleep(time.Millisecond * 40)
		// first gc will be run after 20 secs, so calling it manually
		go c.GC()
		// waiting until all objects are marked as closing
		time.Sleep(time.Millisecond * 40)
		var events []string
		go func() {
			_, err := c.Get(context.TODO(), "id")
			require.NoError(t, err)
			require.NotNil(t, val)
			events = append(events, "get")
			close(getCh)
		}()
		events = append(events, "close")
		// sleeping to make sure that Get is called
		time.Sleep(time.Millisecond * 40)
		close(closeCh)

		<-getCh
		require.Equal(t, []string{"close", "get"}, events)
	})
}

func Test_OCache_Remove(t *testing.T) {
	closeCh := make(chan struct{})
	getCh := make(chan struct{})

	c := New(func(ctx context.Context, id string) (value Object, err error) {
		return NewTestObject(id, closeCh), nil
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
	time.Sleep(time.Millisecond * 40)

	var events []string
	go func() {
		_, err := c.Get(context.TODO(), "id")
		require.NoError(t, err)
		require.NotNil(t, val)
		events = append(events, "get")
		close(getCh)
	}()
	events = append(events, "close")
	// sleeping to make sure that Get is called
	time.Sleep(time.Millisecond * 40)
	close(closeCh)

	<-getCh
	require.Equal(t, []string{"close", "get"}, events)
}
