//go:build !darwin && !linux && !freebsd && !windows

package yamux

import (
	"context"
	"syscall"
)

// abortOnCancel is a no-op on unsupported platforms
func abortOnCancel(ctx context.Context, _ interface{}, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		// No-op on unsupported platforms
	case <-done:
		// Connection established or failed
	}
}

// controlFunc returns nil Control function on unsupported platforms
func controlFunc(ctx context.Context, done <-chan struct{}) func(network, address string, c syscall.RawConn) error {
	// On unsupported platforms, we still need to watch for context cancellation
	// but we can't force-abort the socket
	go abortOnCancel(ctx, nil, done)
	return nil
}