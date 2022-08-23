package remotediff

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ldiff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/space/spacesync"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
)

func NewRemoteDiff(p pool.Pool, peerId, spaceId string) ldiff.Remote {
	return remote{
		pool:    p,
		peerId:  peerId,
		spaceId: spaceId,
	}
}

type remote struct {
	pool    pool.Pool
	peerId  string
	spaceId string
}

func (r remote) Ranges(ctx context.Context, ranges []ldiff.Range, resBuf []ldiff.RangeResult) (results []ldiff.RangeResult, err error) {
	results = resBuf[:0]
	pbRanges := make([]*spacesync.DiffRangeRequestRange, 0, len(ranges))
	for _, rg := range ranges {
		pbRanges = append(pbRanges, &spacesync.DiffRangeRequestRange{
			From:  rg.From,
			To:    rg.To,
			Limit: uint32(rg.Limit),
		})
	}
	req := &spacesync.Space{
		SpaceId: r.spaceId,
		Message: &spacesync.SpaceContent{
			Value: &spacesync.SpaceContentValueOfDiffRange{
				DiffRange: &spacesync.DiffRange{
					Request: &spacesync.DiffRangeRequest{
						Ranges: pbRanges,
					},
				},
			},
		},
	}
	msg, err := req.Marshal()
	if err != nil {
		return
	}
	resp, err := r.pool.SendAndWaitResponse(ctx, r.peerId, &syncproto.Message{
		Header: &syncproto.Header{
			Type: syncproto.MessageType_MessageTypeSpace,
		},
		Data: msg,
	})
	if err != nil {
		return
	}
	var spaceResp = &spacesync.Space{}
	if err = resp.UnmarshalData(spaceResp); err != nil {
		return
	}
	rangeResp := spaceResp.GetMessage().GetDiffRange().GetResponse()
	if rangeResp != nil {
		return nil, fmt.Errorf("got nil response")
	}
	for _, rr := range rangeResp.Results {
		var elms []ldiff.Element
		if len(rr.Elements) > 0 {
			elms = make([]ldiff.Element, 0, len(rr.Elements))
		}
		results = append(results, ldiff.RangeResult{
			Hash:     rr.Hash,
			Elements: elms,
			Count:    int(rr.Count),
		})
	}
	return
}

func HandlerRangeRequest(ctx context.Context, d ldiff.Diff, diffRange *spacesync.DiffRange) (resp *spacesync.DiffRange, err error) {
	req := diffRange.GetRequest()
	if req != nil {
		return nil, fmt.Errorf("received nil request")
	}

	ranges := make([]ldiff.Range, 0, len(req.Ranges))
	for _, reqRange := range req.Ranges {
		ranges = append(ranges, ldiff.Range{
			From:  reqRange.From,
			To:    reqRange.To,
			Limit: int(reqRange.Limit),
		})
	}
	res, err := d.Ranges(ctx, ranges, nil)
	if err != nil {
		return
	}

	var rangeResp = &spacesync.DiffRangeResponse{
		Results: make([]*spacesync.DiffRangeResponseResult, len(res)),
	}
	for _, rangeRes := range res {
		var elements []*spacesync.DiffRangeResponseResultElement
		if len(rangeRes.Elements) > 0 {
			elements = make([]*spacesync.DiffRangeResponseResultElement, 0, len(rangeRes.Elements))
			for _, el := range rangeRes.Elements {
				elements = append(elements, &spacesync.DiffRangeResponseResultElement{
					Id:   el.Id,
					Head: el.Head,
				})
			}
		}
		rangeResp.Results = append(rangeResp.Results, &spacesync.DiffRangeResponseResult{
			Hash:     rangeRes.Hash,
			Elements: elements,
			Count:    uint32(rangeRes.Count),
		})
	}
	return &spacesync.DiffRange{
		Response: rangeResp,
	}, nil
}
