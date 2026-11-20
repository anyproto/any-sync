package recordverifier

import (
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type ValidateFull struct {
	acceptedLog bool
}

func NewValidateFull() RecordVerifier {
	return &ValidateFull{}
}

// NewValidateAcceptedLog validates content in full for a list that ingests only the log the network
// accepted, as a reader of the consensus log does. What a record's author does on the authority of the
// parent space was checked by the coordinator, so such a list accepts it. A list of NewValidateFull
// refuses it.
func NewValidateAcceptedLog() RecordVerifier {
	return &ValidateFull{acceptedLog: true}
}

func (a *ValidateFull) VerifyAcceptor(_ *consensusproto.RawRecord) error {
	return nil
}

func (a *ValidateFull) ShouldValidate() bool {
	return true
}

// AcceptedLog reports that the list ingests only records the network accepted
func (a *ValidateFull) AcceptedLog() bool {
	return a.acceptedLog
}
