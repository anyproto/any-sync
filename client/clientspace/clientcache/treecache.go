package clientcache

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"time"
)

var log = logger.NewNamed("treecache")
var ErrCacheObjectWithoutTree = errors.New("cache object contains no tree")

type ctxKey int

const spaceKey ctxKey = 0

type treeCache struct {
	gcttl         int
	cache         ocache.OCache
	clientService clientspace.Service
}

func New(ttl int) cache.TreeCache {
	return &treeCache{
		gcttl: ttl,
	}
}

func (c *treeCache) Run(ctx context.Context) (err error) {
	return nil
}

func (c *treeCache) Close(ctx context.Context) (err error) {
	return c.cache.Close()
}

func (c *treeCache) Init(a *app.App) (err error) {
	c.clientService = a.MustComponent(clientspace.CName).(clientspace.Service)
	c.cache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			spaceId := ctx.Value(spaceKey).(string)
			space, err := c.clientService.GetSpace(ctx, spaceId)
			if err != nil {
				return
			}
			return space.BuildTree(ctx, id, nil)
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Duration(c.gcttl)*time.Second),
		ocache.WithRefCounter(false),
	)
	return nil
}

func (c *treeCache) Name() (name string) {
	return cache.CName
}

func (c *treeCache) GetTree(ctx context.Context, spaceId, id string) (res cache.TreeResult, err error) {
	var cacheRes ocache.Object
	ctx = context.WithValue(ctx, spaceKey, spaceId)
	cacheRes, err = c.cache.Get(ctx, id)
	if err != nil {
		return cache.TreeResult{}, err
	}

	treeContainer, ok := cacheRes.(cache.TreeContainer)
	if !ok {
		err = ErrCacheObjectWithoutTree
		return
	}

	res = cache.TreeResult{
		Release: func() {
			c.cache.Release(id)
		},
		TreeContainer: treeContainer,
	}
	return
}
