package consensusrpc

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto/consensuserr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/db"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/stream"
	"storj.io/drpc/drpcerr"
	"time"
)

const CName = "consensus.consensusrpc"

func New() app.Component {
	return &consensusRpc{}
}

// consensusRpc implements consensus rpc server
type consensusRpc struct {
	db     db.Service
	stream stream.Service
}

func (c *consensusRpc) Init(a *app.App) (err error) {
	c.db = a.MustComponent(db.CName).(db.Service)
	c.stream = a.MustComponent(stream.CName).(stream.Service)
	return consensusproto.DRPCRegisterConsensus(a.MustComponent(server.CName).(server.DRPCServer), c)
}

func (c *consensusRpc) Name() (name string) {
	return CName
}

func (c *consensusRpc) AddLog(ctx context.Context, req *consensusproto.AddLogRequest) (*consensusproto.Ok, error) {
	if err := c.db.AddLog(ctx, logFromProto(req.Log)); err != nil {
		return nil, err
	}
	return &consensusproto.Ok{}, nil
}

func (c *consensusRpc) AddRecord(ctx context.Context, req *consensusproto.AddRecordRequest) (*consensusproto.Ok, error) {
	if err := c.db.AddRecord(ctx, req.LogId, recordFromProto(req.Record)); err != nil {
		return nil, err
	}
	return &consensusproto.Ok{}, nil
}

func (c *consensusRpc) WatchLog(rpcStream consensusproto.DRPCConsensus_WatchLogStream) error {
	stream := c.stream.NewStream()
	defer stream.Close()
	go c.readStream(stream, rpcStream)
	for {
		recs := stream.WaitLogs()
		if len(recs) == 0 {
			return rpcStream.Close()
		}
		for _, rec := range recs {
			if rec.Err == nil {
				if err := rpcStream.Send(&consensusproto.WatchLogEvent{
					LogId:   rec.Id,
					Records: recordsToProto(rec.Records),
				}); err != nil {
					return err
				}
			} else {
				errCode := consensusproto.ErrCodes(drpcerr.Code(rec.Err))
				if errCode == 0 {
					errCode = consensusproto.ErrCodes(drpcerr.Code(consensuserr.ErrUnexpected))
				}
				if err := rpcStream.Send(&consensusproto.WatchLogEvent{
					LogId: rec.Id,
					Error: &consensusproto.Err{
						Error: errCode,
					},
				}); err != nil {
					return err
				}
			}
		}
	}
}

func (c *consensusRpc) readStream(st *stream.Stream, rpcStream consensusproto.DRPCConsensus_WatchLogStream) {
	defer st.Close()
	for {
		req, err := rpcStream.Recv()
		if err != nil {
			return
		}
		st.UnwatchIds(rpcStream.Context(), req.UnwatchIds)
		st.WatchIds(rpcStream.Context(), req.WatchIds)
	}
}

func logFromProto(log *consensusproto.Log) consensus.Log {
	return consensus.Log{
		Id:      log.Id,
		Records: recordsFromProto(log.Records),
	}
}

func recordsFromProto(recs []*consensusproto.Record) []consensus.Record {
	res := make([]consensus.Record, len(recs))
	for i, rec := range recs {
		res[i] = recordFromProto(rec)
	}
	return res
}

func recordFromProto(rec *consensusproto.Record) consensus.Record {
	return consensus.Record{
		Id:      rec.Id,
		PrevId:  rec.PrevId,
		Payload: rec.Payload,
		Created: time.Unix(int64(rec.CreatedUnix), 0),
	}
}

func recordsToProto(recs []consensus.Record) []*consensusproto.Record {
	res := make([]*consensusproto.Record, len(recs))
	for i, rec := range recs {
		res[i] = &consensusproto.Record{
			Id:          rec.Id,
			PrevId:      rec.PrevId,
			Payload:     rec.Payload,
			CreatedUnix: uint64(rec.Created.Unix()),
		}
	}
	return res
}
