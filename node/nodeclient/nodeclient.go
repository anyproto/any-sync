//go:generate mockgen -destination mock_nodeclient/mock_nodeclient.go github.com/anyproto/any-sync/node/nodeclient NodeClient
package nodeclient

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.node.nodeclient"

func New() NodeClient {
	return &nodeClient{}
}

type NodeClient interface {
	app.Component
	AclGetRecords(ctx context.Context, spaceId, aclHead string) (recs []*consensusproto.RawRecordWithId, err error)
	AclAddRecord(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (recWithId *consensusproto.RawRecordWithId, err error)
}

type nodeClient struct {
	pool     pool.Service
	nodeConf nodeconf.Service
}

func (c *nodeClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Service)
	c.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return
}

func (c *nodeClient) Name() (name string) {
	return CName
}

func (c *nodeClient) AclGetRecords(ctx context.Context, spaceId, aclHead string) (recs []*consensusproto.RawRecordWithId, err error) {
	err = clientDo(c, ctx, spaceId, func(cl spacesyncproto.DRPCSpaceSyncClient) error {
		resp, err := cl.AclGetRecords(ctx, &spacesyncproto.AclGetRecordsRequest{
			SpaceId: spaceId,
			AclHead: aclHead,
		})
		if err != nil {
			return err
		}
		recs = make([]*consensusproto.RawRecordWithId, len(resp.Records))
		for i, rec := range resp.Records {
			recs[i] = &consensusproto.RawRecordWithId{}
			if err = recs[i].Unmarshal(rec); err != nil {
				return err
			}
		}
		return nil
	})
	return
}

func (c *nodeClient) AclAddRecord(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (recWithId *consensusproto.RawRecordWithId, err error) {
	data, err := rec.Marshal()
	if err != nil {
		return
	}
	err = clientDo(c, ctx, spaceId, func(cl spacesyncproto.DRPCSpaceSyncClient) error {
		res, err := cl.AclAddRecord(ctx, &spacesyncproto.AclAddRecordRequest{
			SpaceId: spaceId,
			Payload: data,
		})
		if err != nil {
			return err
		}
		recWithId = &consensusproto.RawRecordWithId{
			Payload: res.Payload,
			Id:      res.RecordId,
		}
		return nil
	})
	return
}

var clientDo = (*nodeClient).doClient

func (c *nodeClient) doClient(ctx context.Context, spaceId string, f func(cl spacesyncproto.DRPCSpaceSyncClient) error) error {
	p, err := c.pool.GetOneOf(ctx, c.nodeConf.NodeIds(spaceId))
	if err != nil {
		return err
	}
	return p.DoDrpc(ctx, func(conn drpc.Conn) error {
		err := f(spacesyncproto.NewDRPCSpaceSyncClient(conn))
		return rpcerr.Unwrap(err)
	})
}
