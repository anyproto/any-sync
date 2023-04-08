//go:generate mockgen -destination mock_coordinatorclient/mock_coordinatorclient.go github.com/anytypeio/any-sync/coordinator/coordinatorclient CoordinatorClient
package coordinatorclient

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/coordinator/coordinatorproto"
	"github.com/anytypeio/any-sync/net/pool"
	"github.com/anytypeio/any-sync/net/rpc/rpcerr"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/crypto"
)

const CName = "common.coordinator.coordinatorclient"

func New() CoordinatorClient {
	return new(coordinatorClient)
}

type CoordinatorClient interface {
	ChangeStatus(ctx context.Context, spaceId string, deleteRaw *treechangeproto.RawTreeChangeWithId) (status *coordinatorproto.SpaceStatusPayload, err error)
	StatusCheck(ctx context.Context, spaceId string) (status *coordinatorproto.SpaceStatusPayload, err error)
	SpaceSign(ctx context.Context, payload SpaceSignPayload) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error)
	FileLimitCheck(ctx context.Context, spaceId string, identity []byte) (limit uint64, err error)
	NetworkConfiguration(ctx context.Context, currentId string) (*coordinatorproto.NetworkConfigurationResponse, error)
	app.Component
}

type SpaceSignPayload struct {
	SpaceId     string
	SpaceHeader []byte
	OldAccount  crypto.PrivKey
	Identity    crypto.PrivKey
}

type coordinatorClient struct {
	pool     pool.Pool
	nodeConf nodeconf.Service
}

func (c *coordinatorClient) ChangeStatus(ctx context.Context, spaceId string, deleteRaw *treechangeproto.RawTreeChangeWithId) (status *coordinatorproto.SpaceStatusPayload, err error) {
	cl, err := c.client(ctx)
	if err != nil {
		return
	}
	resp, err := cl.SpaceStatusChange(ctx, &coordinatorproto.SpaceStatusChangeRequest{
		SpaceId:               spaceId,
		DeletionChangeId:      deleteRaw.GetId(),
		DeletionChangePayload: deleteRaw.GetRawChange(),
	})
	if err != nil {
		err = rpcerr.Unwrap(err)
		return
	}
	status = resp.Payload
	return
}

func (c *coordinatorClient) StatusCheck(ctx context.Context, spaceId string) (status *coordinatorproto.SpaceStatusPayload, err error) {
	cl, err := c.client(ctx)
	if err != nil {
		return
	}
	resp, err := cl.SpaceStatusCheck(ctx, &coordinatorproto.SpaceStatusCheckRequest{
		SpaceId: spaceId,
	})
	if err != nil {
		err = rpcerr.Unwrap(err)
		return
	}
	status = resp.Payload
	return
}

func (c *coordinatorClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Service).NewPool(CName)
	c.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return
}

func (c *coordinatorClient) Name() (name string) {
	return CName
}

func (c *coordinatorClient) SpaceSign(ctx context.Context, payload SpaceSignPayload) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error) {
	cl, err := c.client(ctx)
	if err != nil {
		return
	}
	newRaw, err := payload.Identity.GetPublic().Raw()
	if err != nil {
		return
	}
	newSignature, err := payload.OldAccount.Sign(newRaw)
	if err != nil {
		return
	}
	oldIdentity, err := payload.OldAccount.GetPublic().Marshall()
	if err != nil {
		return
	}
	resp, err := cl.SpaceSign(ctx, &coordinatorproto.SpaceSignRequest{
		SpaceId:              payload.SpaceId,
		Header:               payload.SpaceHeader,
		OldIdentity:          oldIdentity,
		NewIdentitySignature: newSignature,
	})
	if err != nil {
		err = rpcerr.Unwrap(err)
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
		err = rpcerr.Unwrap(err)
		return
	}
	return resp.Limit, nil
}

func (c *coordinatorClient) NetworkConfiguration(ctx context.Context, currentId string) (resp *coordinatorproto.NetworkConfigurationResponse, err error) {
	cl, err := c.client(ctx)
	if err != nil {
		return
	}
	resp, err = cl.NetworkConfiguration(ctx, &coordinatorproto.NetworkConfigurationRequest{
		CurrentId: currentId,
	})
	if err != nil {
		err = rpcerr.Unwrap(err)
		return
	}
	return
}

func (c *coordinatorClient) client(ctx context.Context) (coordinatorproto.DRPCCoordinatorClient, error) {
	p, err := c.pool.GetOneOf(ctx, c.nodeConf.GetLast().CoordinatorPeers())
	if err != nil {
		return nil, err
	}
	return coordinatorproto.NewDRPCCoordinatorClient(p), nil
}
