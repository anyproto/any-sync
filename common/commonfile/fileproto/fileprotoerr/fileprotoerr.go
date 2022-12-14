package fileprotoerr

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
)

var (
	errGroup       = rpcerr.ErrGroup(fileproto.ErrCodes_ErrorOffset)
	ErrUnexpected  = errGroup.Register(fmt.Errorf("unexpected fileproto error"), uint64(fileproto.ErrCodes_Unexpected))
	ErrCIDNotFound = errGroup.Register(fmt.Errorf("CID not found"), uint64(fileproto.ErrCodes_CIDNotFound))
)
