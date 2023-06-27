package consensuserr

import (
	"fmt"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

var (
	errGroup = rpcerr.ErrGroup(consensusproto.ErrCodes_ErrorOffset)

	ErrUnexpected  = errGroup.Register(fmt.Errorf("unexpected consensus error"), uint64(consensusproto.ErrCodes_Unexpected))
	ErrConflict    = errGroup.Register(fmt.Errorf("records conflict"), uint64(consensusproto.ErrCodes_RecordConflict))
	ErrLogExists   = errGroup.Register(fmt.Errorf("log exists"), uint64(consensusproto.ErrCodes_LogExists))
	ErrLogNotFound = errGroup.Register(fmt.Errorf("log not found"), uint64(consensusproto.ErrCodes_LogNotFound))
)
