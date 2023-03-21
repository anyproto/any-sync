package fileprotoerr

import (
	"fmt"
	"github.com/anytypeio/any-sync/commonfile/fileproto"
	"github.com/anytypeio/any-sync/net/rpc/rpcerr"
)

var (
	errGroup            = rpcerr.ErrGroup(fileproto.ErrCodes_ErrorOffset)
	ErrUnexpected       = errGroup.Register(fmt.Errorf("unexpected fileproto error"), uint64(fileproto.ErrCodes_Unexpected))
	ErrCIDNotFound      = errGroup.Register(fmt.Errorf("CID not found"), uint64(fileproto.ErrCodes_CIDNotFound))
	ErrForbidden        = errGroup.Register(fmt.Errorf("forbidden"), uint64(fileproto.ErrCodes_Forbidden))
	ErrSpaceLimitExceed = errGroup.Register(fmt.Errorf("space limit exceed"), uint64(fileproto.ErrCodes_SpaceLimitExceed))
	ErrBlockSizeExceed  = errGroup.Register(fmt.Errorf("block size exceed"), uint64(fileproto.ErrCodes_BlockSizeExceed))
)
