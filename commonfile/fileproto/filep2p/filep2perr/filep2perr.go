package filep2perr

import (
	"fmt"

	"github.com/anyproto/any-sync/commonfile/fileproto/filep2p"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

// errGroup registers the peer-to-peer file WHOLE-RPC (drpc-level) errors in the
// offset-900 block, distinct from v1 filesync's 200 and v2's 800. Mirrors the
// commonfile/fileproto/fileprotoerr and fileprotov2err packages.
var (
	errGroup          = rpcerr.ErrGroup(filep2p.ErrCodes_ErrorOffset)
	ErrUnexpected     = errGroup.Register(fmt.Errorf("unexpected p2p file error"), uint64(filep2p.ErrCodes_Unexpected))
	ErrForbidden      = errGroup.Register(fmt.Errorf("forbidden"), uint64(filep2p.ErrCodes_Forbidden))
	ErrInvalidRequest = errGroup.Register(fmt.Errorf("invalid request"), uint64(filep2p.ErrCodes_InvalidRequest))
	ErrFileNotFound   = errGroup.Register(fmt.Errorf("file not found"), uint64(filep2p.ErrCodes_FileNotFound))
)

// FromCode maps a whole-RPC ErrCodes value to its registered drpc error so
// handlers can uniformly `return filep2perr.FromCode(code)` at the RPC
// boundary. Unknown/zero codes fall back to ErrUnexpected.
func FromCode(code filep2p.ErrCodes) error {
	switch code {
	case filep2p.ErrCodes_Forbidden:
		return ErrForbidden
	case filep2p.ErrCodes_InvalidRequest:
		return ErrInvalidRequest
	case filep2p.ErrCodes_FileNotFound:
		return ErrFileNotFound
	default:
		return ErrUnexpected
	}
}
