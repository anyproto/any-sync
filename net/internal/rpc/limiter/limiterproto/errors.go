package limiterproto

import (
	"errors"

	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(ErrCodes_ErrorOffset)

	ErrLimitExceeded = errGroup.Register(errors.New("rate limit exceeded"), uint64(ErrCodes_RateLimitExceeded))
)
