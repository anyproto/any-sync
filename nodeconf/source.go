package nodeconf

import "context"

const CNameSource = "common.nodeconf.source"

type Source interface {
	GetLast(ctx context.Context) (c Configuration, err error)
}
