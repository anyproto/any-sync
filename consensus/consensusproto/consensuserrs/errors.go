package consensuserrs

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
)

var (
	errGroup = rpcerr.ErrGroup(consensusproto.ErrCodes_ErrorOffset)

	ErrUnexpected  = errGroup.Register(fmt.Errorf("unexpected consensus error"), uint64(consensusproto.ErrCodes_Unexpected))
	ErrConflict    = errGroup.Register(fmt.Errorf("records conflict"), uint64(consensusproto.ErrCodes_RecordConflict))
	ErrLogExists   = errGroup.Register(fmt.Errorf("log exists"), uint64(consensusproto.ErrCodes_LogExists))
	ErrLogNotFound = errGroup.Register(fmt.Errorf("log not found"), uint64(consensusproto.ErrCodes_LogNotFound))
)
