package spacesyncproto

import (
	"errors"
	"github.com/anytypeio/any-sync/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(ErrCodes_ErrorOffset)

	ErrUnexpected           = errGroup.Register(errors.New("unexpected error"), uint64(ErrCodes_Unexpected))
	ErrSpaceMissing         = errGroup.Register(errors.New("space is missing"), uint64(ErrCodes_SpaceMissing))
	ErrSpaceExists          = errGroup.Register(errors.New("space exists"), uint64(ErrCodes_SpaceExists))
	ErrSpaceNotInCache      = errGroup.Register(errors.New("space not in cache"), uint64(ErrCodes_SpaceNotInCache))
	ErrSpaceIsDeleted       = errGroup.Register(errors.New("space is deleted"), uint64(ErrCodes_SpaceIsDeleted))
	ErrPeerIsNotResponsible = errGroup.Register(errors.New("peer is not responsible for space"), uint64(ErrCodes_PeerIsNotResponsible))
	ErrReceiptInvalid       = errGroup.Register(errors.New("space receipt is not valid"), uint64(ErrCodes_ReceiptIsInvalid))
)
