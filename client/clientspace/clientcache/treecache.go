package clientcache

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document/textdocument"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/treegetter"
	"go.uber.org/zap"
	"time"
)

var log = logger.NewNamed("treecache")
var ErrCacheObjectWithoutTree = errors.New("cache object contains no tree")

type ctxKey int

const (
	spaceKey ctxKey = iota
	treeCreateKey
)

type treeCache struct {
	gcttl         int
	cache         ocache.OCache
	account       accountservice.Service
	clientService clientspace.Service
}

type TreeCache interface {
	treegetter.TreeGetter
	treegetter.TreePutter
	GetDocument(ctx context.Context, spaceId, id string) (doc textdocument.TextDocument, err error)
}

type updateListener struct {
}

func (u *updateListener) Update(tree objecttree.ObjectTree) {
	log.With(
		zap.Strings("heads", tree.Heads()),
		zap.String("tree id", tree.Id())).
		Debug("updating tree")
}

func (u *updateListener) Rebuild(tree objecttree.ObjectTree) {
	log.With(
		zap.Strings("heads", tree.Heads()),
		zap.String("tree id", tree.Id())).
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
	c.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	c.cache = ocache.New(
		func(ctx context.Context, id string) (value ocache.Object, err error) {
			spaceId := ctx.Value(spaceKey).(string)
			space, err := c.clientService.GetSpace(ctx, spaceId)
			if err != nil {
				return
			}
			createPayload, exists := ctx.Value(treeCreateKey).(treestorage.TreeStorageCreatePayload)
			if exists {
				return textdocument.CreateTextDocument(ctx, space, createPayload, &updateListener{}, c.account)
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

func (c *treeCache) GetTree(ctx context.Context, spaceId, id string) (tr objecttree.ObjectTree, err error) {
	doc, err := c.GetDocument(ctx, spaceId, id)
	if err != nil {
		return
	}
	// we have to do this trick, otherwise the compiler won't understand that TextDocument conforms to SyncHandler interface
	tr = doc.InnerTree()
	return
}

func (c *treeCache) PutTree(ctx context.Context, spaceId string, payload treestorage.TreeStorageCreatePayload) (ot objecttree.ObjectTree, err error) {
	ctx = context.WithValue(ctx, spaceKey, spaceId)
	ctx = context.WithValue(ctx, treeCreateKey, payload)
	v, err := c.cache.Get(ctx, payload.RootRawChange.Id)
	if err != nil {
		return
	}
	return v.(objecttree.ObjectTree), nil
}

func (c *treeCache) DeleteTree(ctx context.Context, spaceId, treeId string) (err error) {
	tr, err := c.GetTree(ctx, spaceId, treeId)
	if err != nil {
		return
	}
	err = tr.Delete()
	if err != nil {
		return
	}
	_, err = c.cache.Remove(treeId)
	return
}
