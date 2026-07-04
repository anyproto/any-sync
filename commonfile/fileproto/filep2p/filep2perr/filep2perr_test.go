package filep2perr

import (
	"testing"

	// Import the neighbouring error groups so this test binary registers
	// all of them together: rpcerr panics at init if two groups claim the
	// same offset+code, so a green run proves the 900 band does not
	// collide with v1 (200) / v2 (800) or anything else linked here.
	_ "github.com/anyproto/any-sync/commonfile/fileproto/fileprotoerr"
	_ "github.com/anyproto/any-sync/commonfile/fileproto/fileprotov2/fileprotov2err"

	"github.com/anyproto/any-sync/commonfile/fileproto/filep2p"
)

func TestFromCodeMapsEveryCode(t *testing.T) {
	cases := map[filep2p.ErrCodes]error{
		filep2p.ErrCodes_Unexpected:     ErrUnexpected,
		filep2p.ErrCodes_Forbidden:      ErrForbidden,
		filep2p.ErrCodes_InvalidRequest: ErrInvalidRequest,
		filep2p.ErrCodes_FileNotFound:   ErrFileNotFound,
	}
	for code, want := range cases {
		if got := FromCode(code); got != want {
			t.Fatalf("FromCode(%v) = %v, want %v", code, got, want)
		}
	}
	// Unknown codes fall back to Unexpected.
	if got := FromCode(filep2p.ErrCodes(999)); got != ErrUnexpected {
		t.Fatalf("FromCode(unknown) = %v, want ErrUnexpected", got)
	}
}
