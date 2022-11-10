package clientcache

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document/textdocument"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"go.uber.org/zap"
	"time"
)

var log = logger.NewNamed("treecache")
var ErrCacheObjectWithoutTree = errors.New("cache object contains no tree")

type ctxKey int

const spaceKey ctxKey = 0

type treeCache struct {
	gcttl         int
	cache         ocache.OCache
	account       account.Service
	clientService clientspace.Service
}

type TreeCache interface {
	treegetter.TreeGetter
	GetDocument(ctx context.Context, spaceId, id string) (doc textdocument.TextDocument, err error)
}

type updateListener struct {
}

func (u *updateListener) Update(tree tree.ObjectTree) {
	log.With(
		zap.Strings("heads", tree.Heads()),
		zap.String("tree id", tree.ID())).
		Debug("updating tree")
}

func (u *updateListener) Rebuild(tree tree.ObjectTree) {
	log.With(
		zap.Strings("heads", tree.Heads()),
		zap.String("tree id", tree.ID())).
		Debug("rebuilding tree")
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
	c.account = a.MustComponent(account.CName).(account.Service)
	c.cache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			spaceId := ctx.Value(spaceKey).(string)
			space, err := c.clientService.GetSpace(ctx, spaceId)
			if err != nil {
				return
			}
			return textdocument.NewTextDocument(ctx, space, id, &updateListener{}, c.account)
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

func (c *treeCache) GetDocument(ctx context.Context, spaceId, id string) (doc textdocument.TextDocument, err error) {
	ctx = context.WithValue(ctx, spaceKey, spaceId)
	v, err := c.cache.Get(ctx, id)
	if err != nil {
		return
	}
	doc = v.(textdocument.TextDocument)
	return
}

func (c *treeCache) GetTree(ctx context.Context, spaceId, id string) (tr tree.ObjectTree, err error) {
	return c.GetDocument(ctx, spaceId, id)
}
