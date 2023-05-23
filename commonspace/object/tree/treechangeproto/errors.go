package treechangeproto

import (
	"errors"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(ErrorCodes_ErrorOffset)

	ErrUnexpected = errGroup.Register(errors.New("unexpected error"), uint64(ErrorCodes_Unexpected))
	ErrGetTree    = errGroup.Register(errors.New("tree not found"), uint64(ErrorCodes_GetTreeError))
	ErrFullSync   = errGroup.Register(errors.New("full sync request error"), uint64(ErrorCodes_FullSyncRequestError))
)
