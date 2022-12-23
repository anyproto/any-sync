package filestorage

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSyncer_add(t *testing.T) {
	t.Log("start test")
	fx := newPSFixture(t)
	defer fx.Finish(t)
	s := &syncer{ps: fx.proxyStore, done: make(chan struct{})}
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		<-s.done
	}()
	bs := newTestBocks("1", "2")
	require.NoError(t, fx.Add(ctx, bs))
	go func() {
		s.run(ctx)
	}()
	time.Sleep(time.Millisecond * 10)
	bs2 := newTestBocks("3", "4")
	require.NoError(t, fx.Add(ctx, bs2))
	var done = make(chan struct{})
	go func() {
		defer close(done)
		for _, b := range append(bs, bs2...) {
			t.Log("check", b.Cid().String())
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				ab, err := fx.origin.Get(ctx, b.Cid())
				if err == nil {
					assert.Equal(t, b.RawData(), ab.RawData())
					break
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("timeout")
	}
}

func TestSyncer_delete(t *testing.T) {
	fx := newPSFixture(t)
	defer fx.Finish(t)
	s := &syncer{ps: fx.proxyStore, done: make(chan struct{})}
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		<-s.done
	}()
	go func() {
		s.run(ctx)
	}()
	bs := newTestBocks("1", "2")
	require.NoError(t, fx.cache.Add(ctx, bs))
	require.NoError(t, fx.origin.Add(ctx, bs))

	for cid := range fx.cache.(*testStore).store {
		t.Log("cache", cid)
	}
	for cid := range fx.origin.(*testStore).store {
		t.Log("origin", cid)
	}

	for _, b := range bs {
		require.NoError(t, fx.Delete(ctx, b.Cid()))
	}

	var done = make(chan struct{})
	go func() {
		defer close(done)
		for _, b := range bs {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_, err := fx.origin.Get(ctx, b.Cid())
				if err != nil {
					break
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("timeout")
	}
}
