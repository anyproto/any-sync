package aclclient

import (
	"context"
	"fmt"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.acl.aclclient"

type AclInvitingClient interface {
	app.Component
	AclGetRecords(ctx context.Context, spaceId, aclHead string) ([]*consensusproto.RawRecordWithId, error)
	RequestJoin(ctx context.Context, spaceId string, payload list.RequestJoinPayload) error
}

type aclInvitingClient struct {
	nodeConf nodeconf.Service
	pool     pool.Pool
	keys     *accountdata.AccountKeys
}

func NewAclInvitingClient() AclInvitingClient {
	return &aclInvitingClient{}
}

func (c *aclInvitingClient) Name() (name string) {
	return CName
}

func (c *aclInvitingClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Pool)
	c.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	c.keys = a.MustComponent(accountservice.CName).(accountservice.Service).Account()
	return nil
}

func (c *aclInvitingClient) AclGetRecords(ctx context.Context, spaceId, aclHead string) (recs []*consensusproto.RawRecordWithId, err error) {
	var res *spacesyncproto.AclGetRecordsResponse
	err = c.doClient(ctx, aclHead, func(cl spacesyncproto.DRPCSpaceSyncClient) error {
		var err error
		res, err = cl.AclGetRecords(ctx, &spacesyncproto.AclGetRecordsRequest{
			SpaceId: spaceId,
			AclHead: aclHead,
		})
		return err
	})
	if err != nil {
		return
	}
	for _, rec := range res.Records {
		rawRec := &consensusproto.RawRecordWithId{}
		err = rawRec.Unmarshal(rec)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rawRec)
	}
	return
}

func (c *aclInvitingClient) RequestJoin(ctx context.Context, spaceId string, payload list.RequestJoinPayload) (err error) {
	res, err := c.AclGetRecords(ctx, spaceId, "")
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("acl not found")
	}
	storage, err := liststorage.NewInMemoryAclListStorage(res[0].Id, res)
	if err != nil {
		return err
	}
	acl, err := list.BuildAclListWithIdentity(c.keys, storage, list.NoOpAcceptorVerifier{})
	if err != nil {
		return err
	}
	pubIdentity := payload.InviteKey.GetPublic()
	for _, rec := range acl.AclState().JoinRecords() {
		if rec.RequestIdentity.Equals(pubIdentity) {
			// that means that we already requested to join
			return nil
		}
	}
	rec, err := acl.RecordBuilder().BuildRequestJoin(payload)
	if err != nil {
		return
	}
	data, err := rec.Marshal()
	if err != nil {
		return
	}
	return c.doClient(ctx, acl.Id(), func(cl spacesyncproto.DRPCSpaceSyncClient) error {
		_, err = cl.AclAddRecord(ctx, &spacesyncproto.AclAddRecordRequest{
			SpaceId: spaceId,
			Payload: data,
		})
		return err
	})
}

func (c *aclInvitingClient) doClient(ctx context.Context, spaceId string, f func(cl spacesyncproto.DRPCSpaceSyncClient) error) error {
	p, err := c.pool.GetOneOf(ctx, c.nodeConf.NodeIds(spaceId))
	if err != nil {
		return err
	}
	return p.DoDrpc(ctx, func(conn drpc.Conn) error {
		return f(spacesyncproto.NewDRPCSpaceSyncClient(conn))
	})
}
