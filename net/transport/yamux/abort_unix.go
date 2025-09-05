//go:build darwin || linux || freebsd

package yamux

import (
	"context"
	"syscall"

	"golang.org/x/sys/unix"
)

// abortOnCancel watches the context and aborts the socket when canceled
func abortOnCancel(ctx context.Context, fd int, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		// Force abort the socket to unblock any pending operations
		_ = unix.Shutdown(fd, unix.SHUT_RDWR)
		_ = unix.Close(fd)
	case <-done:
		// Connection established or failed, no need to abort
	}
}

// controlFunc returns a Control function that captures the file descriptor
func controlFunc(ctx context.Context, done <-chan struct{}) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var sockErr error
		err := c.Control(func(fd uintptr) {
			// Start goroutine to watch for context cancellation
			go abortOnCancel(ctx, int(fd), done)
		})
		if err != nil {
			return err
		}
		return sockErr
	}
}