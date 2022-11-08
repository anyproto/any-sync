package account

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/account"
)

const CName = "common.account"

type Service interface {
	app.Component
	Account() *account.AccountData
}

type ConfigGetter interface {
	GetAccount() config.Account
}
