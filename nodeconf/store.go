package nodeconf

import "context"

const CNameStore = "common.nodeconf.store"

type Store interface {
	GetLast(ctx context.Context, netId string) (c Configuration, err error)
	SaveLast(ctx context.Context, c Configuration) (err error)
}
