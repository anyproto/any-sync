package spacesyncproto

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(ErrCodes_ErrorOffset)

	ErrUnexpected   = errGroup.Register(errors.New("Unexpected error"), uint64(ErrCodes_Unexpected))
	ErrSpaceMissing = errGroup.Register(errors.New("Space is missing"), uint64(ErrCodes_SpaceMissing))
)
