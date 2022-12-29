package objectsync

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
)

func TestActionQueue_Send(t *testing.T) {
	maxReaders := 41
	maxLen := 93

	queue := NewActionQueue(maxReaders, maxLen).(*actionQueue)
	counter := atomic.Int32{}
	expectedCounter := int32(maxReaders + (maxLen+1)/2 + 1)
	blocker := make(chan struct{}, expectedCounter)
	waiter := make(chan struct{}, expectedCounter)
	increase := func() error {
		counter.Add(1)
		waiter <- struct{}{}
		<-blocker
		return nil
	}

	queue.Run()
	// sending maxReaders messages, so the goroutines will block on `blocker` channel
	for i := 0; i < maxReaders; i++ {
		queue.Send(increase)
	}
	// waiting until they all make progress
	for i := 0; i < maxReaders; i++ {
		<-waiter
	}
	fmt.Println(counter.Load())
	// check that queue is empty
	require.Equal(t, queue.batcher.Len(), 0)
	// making queue to overflow while readers are blocked
	for i := 0; i < maxLen+1; i++ {
		queue.Send(increase)
	}
	// check that queue was halved after overflow
	require.Equal(t, (maxLen+1)/2+1, queue.batcher.Len())
	// unblocking maxReaders waiting + then we should also unblock the new readers to do a bit more readings
	for i := 0; i < int(expectedCounter); i++ {
		blocker <- struct{}{}
	}
	// waiting for all readers to finish adding
	for i := 0; i < int(expectedCounter)-maxReaders; i++ {
		<-waiter
	}
	queue.Close()
	require.Equal(t, expectedCounter, counter.Load())
}
