//go:build windows

package drpcserver

import (
	"errors"
	"net"
	"os"
	"syscall"
)

const (
	_WSAEMFILE    syscall.Errno = 10024
	_WSAENETRESET syscall.Errno = 10052
	_WSAENOBUFS   syscall.Errno = 10055
)

// isTemporary checks if an error is temporary.
// see related go issue for more detail: https://go-review.googlesource.com/c/go/+/208537/
func isTemporary(err error) bool {
	var nErr net.Error
	if !errors.As(err, &nErr) {
		return false
	}

	if nErr.Temporary() {
		return true
	}

	var sErr *os.SyscallError
	if errors.As(err, &sErr) {
		switch sErr.Err {
		case _WSAENETRESET,
			_WSAEMFILE,
			_WSAENOBUFS:
			return true
		}
	}

	return false
}
