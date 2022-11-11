package settingsservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
)

type Service interface {
}

const deletionChangeType = "space.deletionlist"

type service struct {
	account account.Service
}

func New() Service {
	return nil
}

type DeletedDocumentNotifiable interface {
	NotifyDeleted(id string)
}
