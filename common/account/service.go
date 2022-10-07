package account

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
)

const CName = "common.account"

type Service interface {
	app.Component
	Account() *account.AccountData
}

type ConfigGetter interface {
	GetAccount() config.Account
}
