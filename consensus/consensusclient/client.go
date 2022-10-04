package consensusclient

import "github.com/anytypeio/go-anytype-infrastructure-experiments/app"

const CName = "consensus.client"

type Service interface {
	app.Component
}

type service struct {
}
