package pool

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
)

// Results of request collects replies and errors
// Must be closed after usage r.Close()
type Results struct {
	ctx      context.Context
	cancel   func()
	waiterId uint64
	ch       chan Reply
	pool     *pool
}

// Iterate iterates over replies
// if callback will return a non-nil error then iteration stops
func (r *Results) Iterate(callback func(r Reply) (err error)) (err error) {
	if r.ctx == nil || r.ch == nil {
		return fmt.Errorf("results not initialized")
	}
	for {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case m, ok := <-r.ch:
			if ok {
				if err = callback(m); err != nil {
					return err
				}
			} else {
				return
			}
		}
	}
}

// Close cancels iteration and unregister reply handler in the pool
// Required to call to avoid memory leaks
func (r *Results) Close() (err error) {
	r.cancel()
	return r.pool.waiters.Remove(r.waiterId)
}

// Reply presents the result of request executing can be error or result message
type Reply struct {
	PeerInfo peer.Info
	Error    error
	Message  *Message
}
