package deletionservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
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

func deriveDeletionTreePayload(account account.Service, spaceId string) tree.ObjectTreeCreatePayload {
	return tree.ObjectTreeCreatePayload{
		SignKey:    account.Account().SignKey,
		ChangeType: deletionChangeType,
		SpaceId:    spaceId,
		Identity:   account.Account().Identity,
	}
}
