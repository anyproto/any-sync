package syncqueues

import (
	"context"
	"time"

	"golang.org/x/exp/slices"

	accountService "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.util.syncqueues"

var log = logger.NewNamed(CName)

type SyncQueues interface {
	app.ComponentRunnable
	ActionPool(spaceId string) ActionPool
	Limit(spaceId string) *Limit
}

func New() SyncQueues {
	return &syncQueues{}
}

type syncQueues struct {
	limit          *Limit
	pool           ActionPool
	nodeConf       nodeconf.Service
	accountService accountService.Service
}

func (g *syncQueues) Init(a *app.App) (err error) {
	g.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	g.accountService = a.MustComponent(accountService.CName).(accountService.Service)
	var (
		nodeIds        []string
		iAmResponsible bool
		myId           = g.accountService.Account().PeerId
	)
	for _, node := range g.nodeConf.Configuration().Nodes {
		nodeIds = append(nodeIds, node.PeerId)
		if node.PeerId == myId {
			iAmResponsible = true
		}
	}
	g.pool = NewActionPool(time.Second*30, time.Minute, func(peerId string) *replaceableQueue {
		// increase limits between responsible nodes
		if slices.Contains(nodeIds, peerId) && iAmResponsible {
			return newReplaceableQueue(30, 400)
		} else {
			return newReplaceableQueue(10, 100)
		}
	})
	g.limit = NewLimit([]int{20, 15, 10, 5}, []int{200, 400, 800}, nodeIds, 100)
	return
}

func (g *syncQueues) Run(ctx context.Context) (err error) {
	g.pool.Run()
	return
}

func (g *syncQueues) Close(ctx context.Context) (err error) {
	g.pool.Close()
	return
}

func (g *syncQueues) ActionPool(spaceId string) ActionPool {
	return g.pool
}

func (g *syncQueues) Limit(spaceId string) *Limit {
	return g.limit
}

func (g *syncQueues) Name() string {
	return CName
}
