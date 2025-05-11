package recordverifier

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type AlwaysAccept struct{}

func NewAlwaysAccept() RecordVerifier {
	return &AlwaysAccept{}
}

func (a *AlwaysAccept) Init(_ *app.App) error {
	return nil
}

func (a *AlwaysAccept) Name() string {
	return CName
}

func (a *AlwaysAccept) VerifyAcceptor(_ *consensusproto.RawRecord) error {
	return nil
}
