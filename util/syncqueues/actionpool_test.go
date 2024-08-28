package syncqueues

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestRequestPool(t *testing.T) {
	t.Run("parallel, different peer, same object", func(t *testing.T) {
		rp := NewActionPool(time.Minute, time.Minute, func(peerId string) *replaceableQueue {
			return newReplaceableQueue(1, 1)
		})
		rp.Run()
		// we use wait channel to make sure that blocking does not prevent action from being called
		wait := make(chan struct{})
		wg := &sync.WaitGroup{}
		wg.Add(2)
		rp.Add("peerId", "objectId", func(ctx context.Context) {
			wg.Done()
			<-wait
		}, func() {})
		rp.Add("peerId1", "objectId", func(ctx context.Context) {
			wg.Done()
			<-wait
		}, func() {})
		wg.Wait()
		rp.Close()
	})
	t.Run("parallel, same peer, different object", func(t *testing.T) {
		rp := NewActionPool(time.Minute, time.Minute, func(peerId string) *replaceableQueue {
			return newReplaceableQueue(2, 2)
		})
		rp.Run()
		// we use wait channel to make sure that blocking does not prevent action from being called
		wait := make(chan struct{})
		wg := &sync.WaitGroup{}
		wg.Add(2)
		rp.Add("peerId", "objectId", func(ctx context.Context) {
			wg.Done()
			<-wait
		}, func() {})
		rp.Add("peerId", "objectId1", func(ctx context.Context) {
			wg.Done()
			<-wait
		}, func() {})
		wg.Wait()
		rp.Close()
	})
	t.Run("parallel, same peer, same object", func(t *testing.T) {
		rp := NewActionPool(time.Minute, time.Minute, func(peerId string) *replaceableQueue {
			return newReplaceableQueue(2, 2)
		})
		rp.Run()
		// here we are checking that the second action is not called in parallel,
		// when the first action is not finished
		wait := make(chan struct{})
		cnt := atomic.NewBool(false)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		rp.Add("peerId", "objectId", func(ctx context.Context) {
			cnt.Store(true)
			wg.Done()
			<-wait
		}, func() {})
		time.Sleep(100 * time.Millisecond)
		rp.Add("peerId", "objectId", func(ctx context.Context) {
			require.Fail(t, "should not be called")
			wg.Done()
			<-wait
		}, func() {})
		wg.Wait()
		time.Sleep(100 * time.Millisecond)
		require.True(t, cnt.Load())
		rp.Close()
	})
	t.Run("parallel, same peer, different object, replace", func(t *testing.T) {
		rp := NewActionPool(time.Minute, time.Minute, func(peerId string) *replaceableQueue {
			return newReplaceableQueue(1, 3)
		})
		rp.Run()
		// we expect the second action to be replaced
		wait := make(chan struct{})
		cnt := atomic.NewBool(false)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		rp.Add("peerId", "objectId", func(ctx context.Context) {
			<-wait
		}, func() {})
		rp.Add("peerId", "objectId1", func(ctx context.Context) {
			require.Fail(t, "should not be called")
		}, func() {})
		rp.Add("peerId", "objectId1", func(ctx context.Context) {
			cnt.Store(true)
			wg.Done()
		}, func() {})
		close(wait)
		wg.Wait()
		time.Sleep(100 * time.Millisecond)
		require.True(t, cnt.Load())
		rp.Close()
	})
	t.Run("parallel, same peer, different object, try add failed", func(t *testing.T) {
		rp := NewActionPool(time.Minute, time.Minute, func(peerId string) *replaceableQueue {
			return newReplaceableQueue(1, 1)
		})
		rp.Run()
		// we expect try add to fail and call remove action
		wait := make(chan struct{})
		wg := &sync.WaitGroup{}
		wg.Add(2)
		rp.Add("peerId", "objectId", func(ctx context.Context) {
			<-wait
		}, func() {})
		time.Sleep(100 * time.Millisecond)
		rp.Add("peerId", "objectId1", func(ctx context.Context) {
			wg.Done()
		}, func() {
		})
		rp.Add("peerId", "objectId2", func(ctx context.Context) {
		}, func() {
			wg.Done()
		})
		close(wait)
		wg.Wait()
		rp.Close()
	})
	t.Run("gc", func(t *testing.T) {
		rp := NewActionPool(time.Millisecond*20, time.Millisecond*20, func(peerId string) *replaceableQueue {
			return newReplaceableQueue(2, 2)
		})
		rp.Run()
		wg := &sync.WaitGroup{}
		wg.Add(2)
		rp.Add("peerId1", "objectId1", func(ctx context.Context) {
			wg.Done()
		}, func() {})
		rp.Add("peerId2", "objectId2", func(ctx context.Context) {
			wg.Done()
		}, func() {})
		wg.Wait()
		time.Sleep(200 * time.Millisecond)
		require.Empty(t, rp.(*actionPool).queues)
		rp.Close()
	})
}
