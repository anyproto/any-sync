//go:generate mockgen -destination mock_accountservice/mock_accountservice.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice Service
package accountservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/accountdata"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
)

const CName = "common.account"

type Service interface {
	app.Component
	Account() *accountdata.AccountData
}

type ConfigGetter interface {
	GetAccount() config.Account
}
