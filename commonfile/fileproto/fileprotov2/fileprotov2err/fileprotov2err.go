package fileprotov2err

import (
	"fmt"

	"github.com/anyproto/any-sync/commonfile/fileproto/fileprotov2"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

// errGroup registers the v2 WHOLE-RPC (drpc-level) errors in the offset-800
// block, distinct from v1 filesync's 200 block. Only the cross-cutting
// ErrCodes values are registered here; per-item ErrCode outcomes travel in the
// response body and are NOT registered with rpcerr. Mirrors the v1
// commonfile/fileproto/fileprotoerr package.
var (
	errGroup             = rpcerr.ErrGroup(fileprotov2.ErrCodes_ErrorOffset)
	ErrUnexpected        = errGroup.Register(fmt.Errorf("unexpected filenode-v2 error"), uint64(fileprotov2.ErrCodes_Unexpected))
	ErrForbidden         = errGroup.Register(fmt.Errorf("forbidden"), uint64(fileprotov2.ErrCodes_Forbidden))
	ErrInvalidRequest    = errGroup.Register(fmt.Errorf("invalid request"), uint64(fileprotov2.ErrCodes_InvalidRequest))
	ErrQuerySizeExceeded = errGroup.Register(fmt.Errorf("query size exceeded"), uint64(fileprotov2.ErrCodes_QuerySizeExceeded))
)

// FromCode maps a whole-RPC ErrCodes value to its registered drpc error so
// handlers can uniformly `return fileprotov2err.FromCode(code)` at the RPC
// boundary. Unknown/zero codes fall back to ErrUnexpected.
func FromCode(code fileprotov2.ErrCodes) error {
	switch code {
	case fileprotov2.ErrCodes_Forbidden:
		return ErrForbidden
	case fileprotov2.ErrCodes_InvalidRequest:
		return ErrInvalidRequest
	case fileprotov2.ErrCodes_QuerySizeExceeded:
		return ErrQuerySizeExceeded
	default:
		return ErrUnexpected
	}
}
