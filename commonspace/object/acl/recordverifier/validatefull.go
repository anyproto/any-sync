package recordverifier

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type ValidateFull struct{}

func NewValidateFull() RecordVerifier {
	return &ValidateFull{}
}

func (a *ValidateFull) Init(_ *app.App) error {
	return nil
}

func (a *ValidateFull) Name() string {
	return CName
}

func (a *ValidateFull) VerifyAcceptor(_ *consensusproto.RawRecord) error {
	return nil
}

func (a *ValidateFull) ShouldValidate() bool {
	return true
}
