package nodespace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	peer "github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"golang.org/x/exp/slices"
	"time"
)

const CName = "node.nodespace"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	GetSpace(ctx context.Context, id string) (commonspace.Space, error)
	GetOrPickSpace(ctx context.Context, id string) (commonspace.Space, error)
	app.ComponentRunnable
}

type service struct {
	conf                 commonspace.Config
	spaceCache           ocache.OCache
	commonSpace          commonspace.SpaceService
	confService          nodeconf.Service
	spaceStorageProvider spacestorage.SpaceStorageProvider
}

func (s *service) Init(a *app.App) (err error) {
	s.conf = a.MustComponent("config").(commonspace.ConfigGetter).GetSpace()
	s.commonSpace = a.MustComponent(commonspace.CName).(commonspace.SpaceService)
	s.confService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.spaceStorageProvider = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorageProvider)
	s.spaceCache = ocache.New(
		s.loadSpace,
		ocache.WithLogger(log.Sugar()),
		ocache.WithGCPeriod(time.Minute),
		ocache.WithTTL(time.Duration(s.conf.GCTTL)*time.Second),
	)
	return spacesyncproto.DRPCRegisterSpaceSync(a.MustComponent(server.CName).(server.DRPCServer), &rpcHandler{s})
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return
}

func (s *service) GetSpace(ctx context.Context, id string) (commonspace.Space, error) {
	v, err := s.spaceCache.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return v.(commonspace.Space), nil
}

func (s *service) GetOrPickSpace(ctx context.Context, id string) (sp commonspace.Space, err error) {
	var v ocache.Object
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return
	}
	// if we are getting request from our fellow node
	// don't try to wake the space, otherwise it can be possible
	// that node would be waking up infinitely, depending on
	// OCache ttl and HeadSync period
	if slices.Contains(s.confService.GetLast().NodeIds(id), peerId) {
		v, err = s.spaceCache.Pick(ctx, id)
		if err != nil {
			// safely checking that we don't have the space storage
			// this should not open the database in case of node
			if !s.spaceStorageProvider.SpaceExists(id) {
				err = spacesyncproto.ErrSpaceMissing
			}
			return
		}
	} else {
		// if the request is from the client it is safe to wake up the node
		v, err = s.spaceCache.Get(ctx, id)
		if err != nil {
			return
		}
	}
	sp = v.(commonspace.Space)
	return
}

func (s *service) loadSpace(ctx context.Context, id string) (value ocache.Object, err error) {
	cc, err := s.commonSpace.NewSpace(ctx, id)
	if err != nil {
		return
	}
	ns, err := newNodeSpace(cc)
	if err != nil {
		return
	}
	if err = ns.Init(ctx); err != nil {
		return
	}
	return ns, nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.spaceCache.Close()
}
