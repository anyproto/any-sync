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

type causeErr struct{}

func (causeErr) Error() string { return "cause" }

func TestNewConnClosedError(t *testing.T) {
	cause := &causeErr{}
	err := NewConnClosedError(cause)
	assert.ErrorIs(t, err, ErrConnClosed)
	assert.ErrorIs(t, err, net.ErrClosed)
	var got *causeErr
	assert.True(t, errors.As(err, &got), "the original error stays reachable")
	assert.Equal(t, "transport connection closed: cause", err.Error())
	// idempotent
	assert.Equal(t, err, NewConnClosedError(err))
}
