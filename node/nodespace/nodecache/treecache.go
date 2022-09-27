package nodecache

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"time"
)

var log = logger.NewNamed("treecache")
var ErrCacheObjectWithoutTree = errors.New("cache object contains no tree")

type treeCache struct {
	gcttl int
	cache ocache.OCache
}

func NewNodeCache(ttl int) cache.TreeCache {
	return &treeCache{
		gcttl: ttl,
	}
}

func (c *treeCache) SetBuildFunc(buildFunc cache.BuildFunc) {
	c.cache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			return buildFunc(ctx, id, nil)
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Duration(c.gcttl)*time.Second),
		ocache.WithRefCounter(false),
	)
}

func (c *treeCache) Close() (err error) {
	return c.cache.Close()
}

func (c *treeCache) GetTree(ctx context.Context, id string) (res cache.TreeResult, err error) {
	var cacheRes ocache.Object
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
