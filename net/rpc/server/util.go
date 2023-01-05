//go:build !windows

package server

import (
	"errors"
	"net"
)

// isTemporary checks if an error is temporary.
func isTemporary(err error) bool {
	var nErr net.Error
	if errors.As(err, &nErr) {
		return nErr.Temporary()
	}

	return false
}
