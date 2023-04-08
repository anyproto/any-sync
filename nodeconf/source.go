package nodeconf

import (
	"context"
	"errors"
)

const CNameSource = "common.nodeconf.source"

var (
	ErrConfigurationNotChanged = errors.New("configuration not changed")
)

type Source interface {
	GetLast(ctx context.Context, currentId string) (c Configuration, err error)
}
