//go:generate mockgen -destination mock_coordinatorclient/mock_coordinatorclient.go github.com/anyproto/any-sync/coordinator/coordinatorclient CoordinatorClient
package coordinatorclient

import (
	"context"
	"errors"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/crypto"
	"storj.io/drpc"
)

const CName = "common.coordinator.coordinatorclient"

var (
	ErrPubKeyMissing     = errors.New("peer pub key missing")
	ErrNetworkMismatched = errors.New("network mismatched")
)

func New() CoordinatorClient {
	return new(coordinatorClient)
}

type CoordinatorClient interface {
	ChangeStatus(ctx context.Context, spaceId string, conf *coordinatorproto.DeletionConfirmPayloadWithSignature) (status *coordinatorproto.SpaceStatusPayload, err error)
	StatusCheckMany(ctx context.Context, spaceIds []string) (statuses []*coordinatorproto.SpaceStatusPayload, err error)
	StatusCheck(ctx context.Context, spaceId string) (status *coordinatorproto.SpaceStatusPayload, err error)
	SpaceSign(ctx context.Context, payload SpaceSignPayload) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error)
	FileLimitCheck(ctx context.Context, spaceId string, identity []byte) (limit uint64, err error)
	NetworkConfiguration(ctx context.Context, currentId string) (*coordinatorproto.NetworkConfigurationResponse, error)
	DeletionLog(ctx context.Context, lastRecordId string, limit int) (records []*coordinatorproto.DeletionLogRecord, err error)
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

func (c *coordinatorClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Service)
	c.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return
}

func (c *coordinatorClient) Name() (name string) {
	return CName
}

func (c *coordinatorClient) ChangeStatus(ctx context.Context, spaceId string, conf *coordinatorproto.DeletionConfirmPayloadWithSignature) (status *coordinatorproto.SpaceStatusPayload, err error) {
	confMarshalled, err := conf.Marshal()
	if err != nil {
		return nil, err
	}
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.SpaceStatusChange(ctx, &coordinatorproto.SpaceStatusChangeRequest{
			SpaceId:             spaceId,
			DeletionPayload:     confMarshalled,
			DeletionPayloadType: coordinatorproto.DeletionPayloadType_Confirm,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		status = resp.Payload
		return nil
	})
	return
}

func (c *coordinatorClient) DeletionLog(ctx context.Context, lastRecordId string, limit int) (records []*coordinatorproto.DeletionLogRecord, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.DeletionLog(ctx, &coordinatorproto.DeletionLogRequest{
			AfterId: lastRecordId,
			Limit:   uint32(limit),
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		records = resp.Records
		return nil
	})
	return
}

func (c *coordinatorClient) StatusCheckMany(ctx context.Context, spaceIds []string) (statuses []*coordinatorproto.SpaceStatusPayload, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.SpaceStatusCheckMany(ctx, &coordinatorproto.SpaceStatusCheckManyRequest{
			SpaceIds: spaceIds,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		statuses = resp.Payloads
		return nil
	})
	return
}

func (c *coordinatorClient) StatusCheck(ctx context.Context, spaceId string) (status *coordinatorproto.SpaceStatusPayload, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.SpaceStatusCheck(ctx, &coordinatorproto.SpaceStatusCheckRequest{
			SpaceId: spaceId,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		status = resp.Payload
		return nil
	})
	return
}

func (c *coordinatorClient) SpaceSign(ctx context.Context, payload SpaceSignPayload) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error) {
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
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.SpaceSign(ctx, &coordinatorproto.SpaceSignRequest{
			SpaceId:              payload.SpaceId,
			Header:               payload.SpaceHeader,
			OldIdentity:          oldIdentity,
			NewIdentitySignature: newSignature,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		receipt = resp.Receipt
		return nil
	})
	return
}

func (c *coordinatorClient) FileLimitCheck(ctx context.Context, spaceId string, identity []byte) (limit uint64, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.FileLimitCheck(ctx, &coordinatorproto.FileLimitCheckRequest{
			AccountIdentity: identity,
			SpaceId:         spaceId,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		limit = resp.Limit
		return nil
	})
	return
}

func (c *coordinatorClient) NetworkConfiguration(ctx context.Context, currentId string) (resp *coordinatorproto.NetworkConfigurationResponse, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err = cl.NetworkConfiguration(ctx, &coordinatorproto.NetworkConfigurationRequest{
			CurrentId: currentId,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (c *coordinatorClient) doClient(ctx context.Context, f func(cl coordinatorproto.DRPCCoordinatorClient) error) error {
	p, err := c.pool.GetOneOf(ctx, c.nodeConf.CoordinatorPeers())
	if err != nil {
		return err
	}
	pubKey, err := peer.CtxPubKey(p.Context())
	if err != nil {
		return ErrPubKeyMissing
	}
	if pubKey.Network() != c.nodeConf.Configuration().NetworkId {
		return ErrNetworkMismatched
	}
	return p.DoDrpc(ctx, func(conn drpc.Conn) error {
		return f(coordinatorproto.NewDRPCCoordinatorClient(conn))
	})
}
