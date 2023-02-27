//go:generate mockgen -destination mock_coordinatorclient/mock_coordinatorclient.go github.com/anytypeio/any-sync/coordinator/coordinatorclient CoordinatorClient
package coordinatorclient

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/coordinator/coordinatorproto"
	"github.com/anytypeio/any-sync/net/pool"
	"github.com/anytypeio/any-sync/nodeconf"
)

const CName = "common.coordinator.coordinatorclient"

func New() CoordinatorClient {
	return new(coordinatorClient)
}

type CoordinatorClient interface {
	SpaceSign(ctx context.Context, spaceId string) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error)
	FileLimitCheck(ctx context.Context, spaceId string, identity []byte) (limit uint64, err error)
	app.Component
}

type coordinatorClient struct {
	pool     pool.Pool
	nodeConf nodeconf.Service
}

func (c *coordinatorClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Service).NewPool(CName)
	c.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return
}

func (c *coordinatorClient) Name() (name string) {
	return CName
}

func (c *coordinatorClient) SpaceSign(ctx context.Context, spaceId string) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error) {
	cl, err := c.client(ctx)
	if err != nil {
		return
	}
	resp, err := cl.SpaceSign(ctx, &coordinatorproto.SpaceSignRequest{
		SpaceId: spaceId,
	})
	if err != nil {
		return
	}
	return resp.Receipt, nil
}

func (c *coordinatorClient) FileLimitCheck(ctx context.Context, spaceId string, identity []byte) (limit uint64, err error) {
	cl, err := c.client(ctx)
	if err != nil {
		return
	}
	resp, err := cl.FileLimitCheck(ctx, &coordinatorproto.FileLimitCheckRequest{
		AccountIdentity: identity,
		SpaceId:         spaceId,
	})
	if err != nil {
		return
	}
	return resp.Limit, nil
}

func (c *coordinatorClient) client(ctx context.Context) (coordinatorproto.DRPCCoordinatorClient, error) {
	p, err := c.pool.GetOneOf(ctx, c.nodeConf.GetLast().CoordinatorPeers())
	if err != nil {
		return nil, err
	}
	return coordinatorproto.NewDRPCCoordinatorClient(p), nil
}
