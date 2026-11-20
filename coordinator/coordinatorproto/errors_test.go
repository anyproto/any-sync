package coordinatorproto

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/drpc/drpcerr"

	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

// A client gets back only the code of an error, so each refusal needs a code of its own to stay
// distinguishable on the other side.
func TestParentChildErrorsKeepTheirCode(t *testing.T) {
	for code, registered := range map[ErrorCodes]error{
		ErrorCodes_ParentSpaceNotAvailable: ErrParentSpaceNotAvailable,
		ErrorCodes_ParentSpaceInvalid:      ErrParentSpaceInvalid,
		ErrorCodes_ParentPermissionDenied:  ErrParentPermissionDenied,
		ErrorCodes_ChildSpacesLimitReached: ErrChildSpacesLimitReached,
		ErrorCodes_NotParentMember:         ErrNotParentMember,
		ErrorCodes_SpaceHasChildren:        ErrSpaceHasChildren,
	} {
		wire := drpcerr.WithCode(errors.New("remote"), uint64(ErrorCodes_ErrorOffset)+uint64(code))
		require.ErrorIs(t, rpcerr.Unwrap(wire), registered, code.String())
		require.NotErrorIs(t, registered, ErrForbidden, code.String())
	}
}
