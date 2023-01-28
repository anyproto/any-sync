package multiqueue

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMultiQueue_Add(t *testing.T) {
	t.Run("process", func(t *testing.T) {
		var msgsCh = make(chan string)
		var h HandleFunc[string] = func(msg string) {
			msgsCh <- msg
		}
		q := New[string](h, 10)
		defer func() {
			require.NoError(t, q.Close())
		}()

		for i := 0; i < 5; i++ {
			for j := 0; j < 5; j++ {
				assert.NoError(t, q.Add(context.Background(), fmt.Sprint(i), fmt.Sprint(i, j)))
			}
		}
		var msgs []string
		for i := 0; i < 5*5; i++ {
			select {
			case <-time.After(time.Second / 4):
				require.True(t, false, "timeout")
			case msg := <-msgsCh:
				msgs = append(msgs, msg)
			}
		}
		assert.Len(t, msgs, 25)
	})
	t.Run("add to closed", func(t *testing.T) {
		q := New[string](func(msg string) {}, 10)
		require.NoError(t, q.Close())
		assert.Equal(t, ErrClosed, q.Add(context.Background(), "1", "1"))
	})
}

func TestMultiQueue_CloseThread(t *testing.T) {
	var msgsCh = make(chan string)
	var h HandleFunc[string] = func(msg string) {
		msgsCh <- msg
	}
	q := New[string](h, 10)
	defer func() {
		require.NoError(t, q.Close())
	}()
	require.NoError(t, q.Add(context.Background(), "1", "1"))
	require.NoError(t, q.Add(context.Background(), "1", "2"))
	require.NoError(t, q.CloseThread("1"))
	for i := 0; i < 2; i++ {
		select {
		case <-msgsCh:
		case <-time.After(time.Second / 4):
			require.False(t, true, "timeout")
		}
	}
}
