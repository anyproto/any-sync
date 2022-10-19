package clientcache

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"time"
)

var log = logger.NewNamed("treecache")
var ErrCacheObjectWithoutTree = errors.New("cache object contains no tree")

type ctxKey int

const spaceKey ctxKey = 0

type treeCache struct {
	gcttl         int
	cache         ocache.OCache
	docService    document.Service
	clientService clientspace.Service
}

type TreeCache interface {
	treegetter.TreeGetter
	GetDocument(ctx context.Context, spaceId, id string) (doc document.TextDocument, err error)
}

func New(ttl int) TreeCache {
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
	c.docService = a.MustComponent(document.CName).(document.Service)
	c.cache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			spaceId := ctx.Value(spaceKey).(string)
			space, err := c.clientService.GetSpace(ctx, spaceId)
			if err != nil {
				return
			}
			return document.NewTextDocument(context.Background(), space, id, c.docService)
		},
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Duration(c.gcttl)*time.Second),
	)
	return nil
}

func (c *treeCache) Name() (name string) {
	return treegetter.CName
}

func (c *treeCache) GetDocument(ctx context.Context, spaceId, id string) (doc document.TextDocument, err error) {
	ctx = context.WithValue(ctx, spaceKey, spaceId)
	v, err := c.cache.Get(ctx, id)
	if err != nil {
		return
	}
	doc = v.(document.TextDocument)
	return
}

func (c *treeCache) GetTree(ctx context.Context, spaceId, id string) (tr tree.ObjectTree, err error) {
	doc, err := c.GetDocument(ctx, spaceId, id)
	if err != nil {
		return
	}
	tr = doc.Tree()
	return
}
