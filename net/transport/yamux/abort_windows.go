//go:build windows

package yamux

import (
	"context"
	"syscall"

	"golang.org/x/sys/windows"
)

// abortOnCancel watches the context and aborts the socket when canceled
func abortOnCancel(ctx context.Context, h windows.Handle, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		// Force abort the socket to unblock any pending operations
		_ = windows.CancelIoEx(h, nil)
		_ = windows.Closesocket(h)
	case <-done:
		// Connection established or failed, no need to abort
	}
}

// controlFunc returns a Control function that captures the socket handle
func controlFunc(ctx context.Context, done <-chan struct{}) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var sockErr error
		err := c.Control(func(fd uintptr) {
			// Start goroutine to watch for context cancellation
			go abortOnCancel(ctx, windows.Handle(fd), done)
		})
		if err != nil {
			return err
		}
		return sockErr
	}
}