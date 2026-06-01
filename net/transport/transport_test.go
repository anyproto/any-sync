package transport

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrConnClosed(t *testing.T) {
	// must unwrap to net.ErrClosed so the quic idle-timeout normalization keeps
	// errors.Is(err, net.ErrClosed) detectors working across the boundary
	assert.True(t, errors.Is(ErrConnClosed, net.ErrClosed))
	// distinctive text so it can be told apart from other connection errors in logs
	assert.Equal(t, "transport connection closed", ErrConnClosed.Error())
}
