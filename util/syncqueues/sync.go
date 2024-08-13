package syncqueues

import (
	"context"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.util.syncqueues"

var log = logger.NewNamed(CName)

type SyncQueues interface {
	app.ComponentRunnable
	RequestPool(spaceId string) RequestPool
	Limit(spaceId string) *Limit
}

func New() SyncQueues {
	return &syncQueues{}
}

type syncQueues struct {
	limit    *Limit
	rp       RequestPool
	nodeConf nodeconf.Service
}

func (g *syncQueues) Init(a *app.App) (err error) {
	g.rp = NewRequestPool(time.Second*30, time.Minute)
	g.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	var nodeIds []string
	for _, node := range g.nodeConf.Configuration().Nodes {
		nodeIds = append(nodeIds, node.PeerId)
	}
	g.limit = NewLimit([]int{20, 15, 10, 5}, []int{200, 400, 800}, nodeIds, 100)
	return
}

func (g *syncQueues) Run(ctx context.Context) (err error) {
	g.rp.Run()
	return
}

func (g *syncQueues) Close(ctx context.Context) (err error) {
	g.rp.Close()
	return
}

func (g *syncQueues) RequestPool(spaceId string) RequestPool {
	return g.rp
}

func (g *syncQueues) Limit(spaceId string) *Limit {
	return g.limit
}

func (g *syncQueues) Name() string {
	return CName
}
