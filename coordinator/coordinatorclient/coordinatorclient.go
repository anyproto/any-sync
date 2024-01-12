//go:generate mockgen -destination mock_coordinatorclient/mock_coordinatorclient.go github.com/anyproto/any-sync/coordinator/coordinatorclient CoordinatorClient
package coordinatorclient

import (
	"context"
	"errors"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/identityrepo/identityrepoproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
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
	SpaceDelete(ctx context.Context, spaceId string, conf *coordinatorproto.DeletionConfirmPayloadWithSignature) (err error)
	AccountDelete(ctx context.Context, conf *coordinatorproto.DeletionConfirmPayloadWithSignature) (timestamp int64, err error)
	AccountRevertDeletion(ctx context.Context) (err error)
	StatusCheckMany(ctx context.Context, spaceIds []string) (statuses []*coordinatorproto.SpaceStatusPayload, err error)
	StatusCheck(ctx context.Context, spaceId string) (status *coordinatorproto.SpaceStatusPayload, err error)
	SpaceSign(ctx context.Context, payload SpaceSignPayload) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error)
	FileLimitCheck(ctx context.Context, spaceId string, identity []byte) (response *coordinatorproto.FileLimitCheckResponse, err error)
	NetworkConfiguration(ctx context.Context, currentId string) (*coordinatorproto.NetworkConfigurationResponse, error)
	DeletionLog(ctx context.Context, lastRecordId string, limit int) (records []*coordinatorproto.DeletionLogRecord, err error)

	IdentityRepoPut(ctx context.Context, identity string, data []*identityrepoproto.Data) (err error)
	IdentityRepoGet(ctx context.Context, identities []string, kinds []string) (res []*identityrepoproto.DataWithIdentity, err error)
	app.Component
}

type SpaceSignPayload struct {
	SpaceId      string
	SpaceHeader  []byte
	OldAccount   crypto.PrivKey
	Identity     crypto.PrivKey
	ForceRequest bool
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

func (c *coordinatorClient) SpaceDelete(ctx context.Context, spaceId string, conf *coordinatorproto.DeletionConfirmPayloadWithSignature) (err error) {
	confMarshalled, err := conf.Marshal()
	if err != nil {
		return err
	}
	id, err := cidutil.NewCidFromBytes(confMarshalled)
	if err != nil {
		return err
	}
	req := &coordinatorproto.SpaceDeleteRequest{
		SpaceId:           spaceId,
		DeletionPayload:   confMarshalled,
		DeletionPayloadId: id,
	}
	return c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		_, err := cl.SpaceDelete(ctx, req)
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
}

func (c *coordinatorClient) AccountDelete(ctx context.Context, conf *coordinatorproto.DeletionConfirmPayloadWithSignature) (timestamp int64, err error) {
	confMarshalled, err := conf.Marshal()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(confMarshalled)
	if err != nil {
		return
	}
	req := &coordinatorproto.AccountDeleteRequest{
		DeletionPayload:   confMarshalled,
		DeletionPayloadId: id,
	}
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.AccountDelete(ctx, req)
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		timestamp = resp.ToBeDeletedTimestamp
		return nil
	})
	return
}

func (c *coordinatorClient) AccountRevertDeletion(ctx context.Context) (err error) {
	req := &coordinatorproto.AccountRevertDeletionRequest{}
	return c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		_, err := cl.AccountRevertDeletion(ctx, req)
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
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
			ForceRequest:         payload.ForceRequest,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		receipt = resp.Receipt
		return nil
	})
	return
}

func (c *coordinatorClient) FileLimitCheck(ctx context.Context, spaceId string, identity []byte) (resp *coordinatorproto.FileLimitCheckResponse, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err = cl.FileLimitCheck(ctx, &coordinatorproto.FileLimitCheckRequest{
			AccountIdentity: identity,
			SpaceId:         spaceId,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
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

func (c *coordinatorClient) IdentityRepoPut(ctx context.Context, identity string, data []*identityrepoproto.Data) (err error) {
	err = c.doIdentityRepoClient(ctx, func(cl identityrepoproto.DRPCIdentityRepoClient) error {
		_, err := cl.DataPut(ctx, &identityrepoproto.DataPutRequest{
			Identity: identity,
			Data:     data,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (c *coordinatorClient) IdentityRepoGet(ctx context.Context, identities, kinds []string) (res []*identityrepoproto.DataWithIdentity, err error) {
	err = c.doIdentityRepoClient(ctx, func(cl identityrepoproto.DRPCIdentityRepoClient) error {
		resp, err := cl.DataPull(ctx, &identityrepoproto.DataPullRequest{
			Identities: identities,
			Kinds:      kinds,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		res = resp.GetData()
		return nil
	})
	return
}

func (c *coordinatorClient) doClient(ctx context.Context, f func(cl coordinatorproto.DRPCCoordinatorClient) error) error {
	p, err := c.getPeer(ctx)
	if err != nil {
		return err
	}
	return p.DoDrpc(ctx, func(conn drpc.Conn) error {
		return f(coordinatorproto.NewDRPCCoordinatorClient(conn))
	})
}

func (c *coordinatorClient) doIdentityRepoClient(ctx context.Context, f func(cl identityrepoproto.DRPCIdentityRepoClient) error) error {
	p, err := c.getPeer(ctx)
	if err != nil {
		return err
	}
	return p.DoDrpc(ctx, func(conn drpc.Conn) error {
		return f(identityrepoproto.NewDRPCIdentityRepoClient(conn))
	})
}

func (c *coordinatorClient) getPeer(ctx context.Context) (peer.Peer, error) {
	p, err := c.pool.GetOneOf(ctx, c.nodeConf.CoordinatorPeers())
	if err != nil {
		return nil, err
	}
	pubKey, err := peer.CtxPubKey(p.Context())
	if err != nil {
		return nil, ErrPubKeyMissing
	}
	if pubKey.Network() != c.nodeConf.Configuration().NetworkId {
		return nil, ErrNetworkMismatched
	}
	return p, nil
}
