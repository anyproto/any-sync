package globalsync

import (
	"context"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.commonspace.globalsync"

var log = logger.NewNamed(CName)

type GlobalSync interface {
	app.ComponentRunnable
	RequestPool(spaceId string) RequestPool
	Limit(spaceId string) *Limit
}

func New() GlobalSync {
	return &globalSync{}
}

type globalSync struct {
	limit    *Limit
	rp       RequestPool
	nodeConf nodeconf.Service
}

func (g *globalSync) Init(a *app.App) (err error) {
	g.rp = NewRequestPool(time.Second*30, time.Minute)
	g.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	var nodeIds []string
	for _, node := range g.nodeConf.Configuration().Nodes {
		nodeIds = append(nodeIds, node.PeerId)
	}
	g.limit = NewLimit([]int{20, 15, 10, 5}, []int{200, 400, 800}, nodeIds, 100)
	return
}

func (g *globalSync) Run(ctx context.Context) (err error) {
	g.rp.Run()
	return
}

func (g *globalSync) Close(ctx context.Context) (err error) {
	g.rp.Close()
	return
}

func (g *globalSync) RequestPool(spaceId string) RequestPool {
	return g.rp
}

func (g *globalSync) Limit(spaceId string) *Limit {
	return g.limit
}

func (g *globalSync) Name() string {
	return CName
}
