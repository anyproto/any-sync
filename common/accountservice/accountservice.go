//go:generate mockgen -destination mock_accountservice/mock_accountservice.go github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice Service
package accountservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/accountdata"
)

const CName = "common.accountservice"

type Service interface {
	app.Component
	Account() *accountdata.AccountData
}

type Config struct {
	PeerId        string `yaml:"peerId"`
	PeerKey       string `yaml:"peerKey"`
	SigningKey    string `yaml:"signingKey"`
	EncryptionKey string `yaml:"encryptionKey"`
}

type ConfigGetter interface {
	GetAccount() Config
}
