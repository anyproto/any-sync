//go:generate mockgen -destination mock_coordinatorclient/mock_coordinatorclient.go github.com/anyproto/any-sync/coordinator/coordinatorclient CoordinatorClient
package coordinatorclient

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/identityrepo/identityrepoproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

const CName = "common.coordinator.coordinatorclient"

var log = logger.NewNamed(CName)

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
	StatusCheckMany(ctx context.Context, spaceIds []string) (statuses []*coordinatorproto.SpaceStatusPayload, limits *coordinatorproto.AccountLimits, err error)
	StatusCheck(ctx context.Context, spaceId string) (status *coordinatorproto.SpaceStatusPayload, err error)
	SpaceSign(ctx context.Context, payload SpaceSignPayload) (receipt *coordinatorproto.SpaceReceiptWithSignature, err error)
	SpaceMakeShareable(ctx context.Context, spaceId string) (err error)
	SpaceMakeUnshareable(ctx context.Context, spaceId, aclId string) (err error)
	NetworkConfiguration(ctx context.Context, currentId string) (*coordinatorproto.NetworkConfigurationResponse, error)
	IsNetworkNeedsUpdate(ctx context.Context) (bool, error)
	DeletionLog(ctx context.Context, lastRecordId string, limit int) (records []*coordinatorproto.DeletionLogRecord, err error)

	IdentityRepoPut(ctx context.Context, identity string, data []*identityrepoproto.Data) (err error)
	IdentityRepoGet(ctx context.Context, identities []string, kinds []string) (res []*identityrepoproto.DataWithIdentity, err error)

	AclAddRecord(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (res *consensusproto.RawRecordWithId, err error)
	AclGetRecords(ctx context.Context, spaceId, aclHead string) (res []*consensusproto.RawRecordWithId, err error)

	AccountLimitsSet(ctx context.Context, req *coordinatorproto.AccountLimitsSetRequest) error

	AclEventLog(ctx context.Context, accountId, lastRecordId string, limit int) (records []*coordinatorproto.AclEventLogRecord, err error)
	InboxFetch(ctx context.Context, accountId, offset string) (records []*coordinatorproto.InboxMessage, err error)

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

	stream *stream
	mu     sync.Mutex
}

func (c *coordinatorClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Service)
	c.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return
}

func (c *coordinatorClient) Name() (name string) {
	return CName
}

func (c *coordinatorClient) Run(_ context.Context) error {
	go c.streamWatcher()
	return nil
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

func (c *coordinatorClient) StatusCheckMany(ctx context.Context, spaceIds []string) (
	statuses []*coordinatorproto.SpaceStatusPayload,
	limits *coordinatorproto.AccountLimits,
	err error,
) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.SpaceStatusCheckMany(ctx, &coordinatorproto.SpaceStatusCheckManyRequest{
			SpaceIds: spaceIds,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		statuses = resp.Payloads
		limits = resp.AccountLimits
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

func (c *coordinatorClient) AclAddRecord(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (res *consensusproto.RawRecordWithId, err error) {
	recordData, err := rec.Marshal()
	if err != nil {
		return
	}
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.AclAddRecord(ctx, &coordinatorproto.AclAddRecordRequest{
			SpaceId: spaceId,
			Payload: recordData,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		res = &consensusproto.RawRecordWithId{
			Payload: resp.Payload,
			Id:      resp.RecordId,
		}
		return nil
	})
	return
}

func (c *coordinatorClient) AclGetRecords(ctx context.Context, spaceId, aclHead string) (res []*consensusproto.RawRecordWithId, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.AclGetRecords(ctx, &coordinatorproto.AclGetRecordsRequest{
			SpaceId: spaceId,
			AclHead: aclHead,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		res = make([]*consensusproto.RawRecordWithId, len(resp.Records))
		for i, rec := range resp.Records {
			res[i] = &consensusproto.RawRecordWithId{}
			if err = res[i].Unmarshal(rec); err != nil {
				return err
			}
		}
		return nil
	})
	return
}

func (c *coordinatorClient) AccountLimitsSet(ctx context.Context, req *coordinatorproto.AccountLimitsSetRequest) error {
	return c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		if _, err := cl.AccountLimitsSet(ctx, req); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
}

func (c *coordinatorClient) SpaceMakeShareable(ctx context.Context, spaceId string) (err error) {
	return c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		if _, err := cl.SpaceMakeShareable(ctx, &coordinatorproto.SpaceMakeShareableRequest{
			SpaceId: spaceId,
		}); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
}

func (c *coordinatorClient) SpaceMakeUnshareable(ctx context.Context, spaceId, aclHead string) (err error) {
	return c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		if _, err := cl.SpaceMakeUnshareable(ctx, &coordinatorproto.SpaceMakeUnshareableRequest{
			SpaceId: spaceId,
			AclHead: aclHead,
		}); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
}

func (c *coordinatorClient) AclEventLog(ctx context.Context, accountId, lastRecordId string, limit int) (records []*coordinatorproto.AclEventLogRecord, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.AclEventLog(ctx, &coordinatorproto.AclEventLogRequest{
			AccountIdentity: accountId,
			AfterId:         lastRecordId,
			Limit:           uint32(limit),
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		records = resp.Records
		return nil
	})
	return
}

func (c *coordinatorClient) InboxFetch(ctx context.Context, accountId, offset string) (messages []*coordinatorproto.InboxMessage, err error) {
	err = c.doClient(ctx, func(cl coordinatorproto.DRPCCoordinatorClient) error {
		resp, err := cl.InboxFetch(ctx, &coordinatorproto.InboxFetchRequest{
			Offset: offset,
		})
		if err != nil {
			return rpcerr.Unwrap(err)
		}
		messages = resp.Messages
		return nil
	})
	return

}

func (c *coordinatorClient) openStream(ctx context.Context) (st *stream, err error) {
	pr, err := c.pool.GetOneOf(ctx, c.nodeConf.CoordinatorPeers())
	if err != nil {
		return nil, err
	}
	pr.SetTTL(time.Hour * 24)
	dc, err := pr.AcquireDrpcConn(ctx)
	if err != nil {
		return nil, err
	}
	rpcStream, err := coordinatorproto.NewDRPCCoordinatorClient(dc).InboxNotifySubscribe(ctx)
	if err != nil {
		return nil, rpcerr.Unwrap(err)
	}
	return runStream(rpcStream), nil
}

func (c *coordinatorClient) streamWatcher() {
	var (
		err error
		st  *stream
	)
	for {
		// open stream
		if st, err = c.openStream(context.Background()); err != nil {
			log.Error("failed to open inbox notification stream", zap.Error(err))
			return
		}
		c.mu.Lock()
		c.stream = st
		c.mu.Unlock()
		// read stream
		if err = c.streamReader(); err != nil {
			log.Error("stream read error", zap.Error(err))
			continue
		}
		return
	}
}

func (c *coordinatorClient) streamReader() error {
	for {
		events := c.stream.WaitNotifyEvents()
		if len(events) == 0 {
			return c.stream.Err()
		}
		c.mu.Lock()
		for _, e := range events {
			// TODO: notify exec
			log.Info("notify event", zap.String("event id", e.NotifyId))
		}
		c.mu.Unlock()
	}
}

func (c *coordinatorClient) IsNetworkNeedsUpdate(ctx context.Context) (bool, error) {
	p, err := c.getPeer(ctx)
	if err != nil {
		return false, err
	}
	version, err := peer.CtxProtoVersion(p.Context())
	if err != nil {
		return false, err
	}
	return secureservice.ProtoVersion < version, nil
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
