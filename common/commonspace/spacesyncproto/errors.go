package spacesyncproto

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(ErrCodes_ErrorOffset)

	ErrUnexpected      = errGroup.Register(errors.New("unexpected error"), uint64(ErrCodes_Unexpected))
	ErrSpaceMissing    = errGroup.Register(errors.New("space is missing"), uint64(ErrCodes_SpaceMissing))
	ErrSpaceExists     = errGroup.Register(errors.New("space exists"), uint64(ErrCodes_SpaceExists))
	ErrSpaceNotInCache = errGroup.Register(errors.New("space not in cache"), uint64(ErrCodes_SpaceNotInCache))
)
