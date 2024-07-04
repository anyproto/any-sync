package sync

import (
	"sync"
	"testing"
	"time"
)

func TestLimit(t *testing.T) {
	l := NewLimit(3)
	ids := []string{"id1", "id2", "id3"}

	var wg sync.WaitGroup
	wg.Add(len(ids) * 5)

	// Function to simulate taking and releasing tokens
	testFunc := func(i int, id string) {
		defer wg.Done()

		start := time.Now()
		l.Take(id)
		taken := time.Since(start)

		// Ensure no more than the limit of tokens are taken simultaneously
		if taken > time.Second {
			t.Errorf("Goroutine %d for %s waited too long to take a token", i, id)
		}

		time.Sleep(500 * time.Millisecond)
		l.Release(id)
	}

	for i := 0; i < 5; i++ {
		for _, id := range ids {
			go testFunc(i, id)
		}
	}

	wg.Wait()

	// Ensure all tokens are released
	for _, id := range ids {
		if l.tokens[id] != 0 {
			t.Errorf("Tokens for %s should be 0, got %d", id, l.tokens[id])
		}
	}
}
