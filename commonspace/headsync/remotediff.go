package headsync

import (
	"bytes"
	"context"
	"encoding/hex"
	"math"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type Client interface {
	HeadSync(ctx context.Context, in *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error)
}

type RemoteDiff interface {
	ldiff.RemoteTypeChecker
	ldiff.Remote
}

func NewRemoteDiff(spaceId string, client Client) RemoteDiff {
	return &remote{
		spaceId: spaceId,
		client:  client,
	}
}

type remote struct {
	spaceId  string
	client   Client
	diffType spacesyncproto.DiffType
}

func (r *remote) Ranges(ctx context.Context, ranges []ldiff.Range, resBuf []ldiff.RangeResult) (results []ldiff.RangeResult, err error) {
	results = resBuf[:0]
	pbRanges := make([]*spacesyncproto.HeadSyncRange, 0, len(ranges))
	for _, rg := range ranges {
		pbRanges = append(pbRanges, &spacesyncproto.HeadSyncRange{
			From:     rg.From,
			To:       rg.To,
			Elements: rg.Elements,
			Limit:    uint32(rg.Limit),
		})
	}
	req := &spacesyncproto.HeadSyncRequest{
		SpaceId:  r.spaceId,
		Ranges:   pbRanges,
		DiffType: r.diffType,
	}
	resp, err := r.client.HeadSync(ctx, req)
	if err != nil {
		return
	}
	for _, rr := range resp.Results {
		var elms []ldiff.Element
		if len(rr.Elements) > 0 {
			elms = make([]ldiff.Element, 0, len(rr.Elements))
		}
		for _, e := range rr.Elements {
			elms = append(elms, ldiff.Element{
				Id:   e.Id,
				Head: e.Head,
			})
		}
		results = append(results, ldiff.RangeResult{
			Hash:     rr.Hash,
			Elements: elms,
			Count:    int(rr.Count),
		})
	}
	return
}

func (r *remote) DiffTypeCheck(ctx context.Context, diffContainer ldiff.DiffContainer) (needsSync bool, diff ldiff.Diff, err error) {
	req := &spacesyncproto.HeadSyncRequest{
		SpaceId:  r.spaceId,
		DiffType: spacesyncproto.DiffType_V3,
		Ranges:   []*spacesyncproto.HeadSyncRange{{From: 0, To: math.MaxUint64}},
	}
	resp, err := r.client.HeadSync(ctx, req)
	if err != nil {
		return
	}
	needsSync = true
	checkHash := func(diff ldiff.Diff) (bool, error) {
		hashB, err := hex.DecodeString(diff.Hash())
		if err != nil {
			return false, err
		}
		if len(resp.Results) != 0 && bytes.Equal(hashB, resp.Results[0].Hash) {
			return false, nil
		}
		return true, nil
	}
	r.diffType = resp.DiffType
	switch resp.DiffType {
	case spacesyncproto.DiffType_V3:
		diff = diffContainer.NewDiff()
		needsSync, err = checkHash(diff)
	case spacesyncproto.DiffType_V2:
		diff = diffContainer.OldDiff()
		needsSync, err = checkHash(diff)
	default:
		err = spacesyncproto.ErrUnexpected
	}
	return
}

func HandleRangeRequest(ctx context.Context, d ldiff.Diff, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	ranges := make([]ldiff.Range, 0, len(req.Ranges))
	// basically we gather data applicable for both diffs
	for _, reqRange := range req.Ranges {
		ranges = append(ranges, ldiff.Range{
			From:     reqRange.From,
			To:       reqRange.To,
			Limit:    int(reqRange.Limit),
			Elements: reqRange.Elements,
		})
	}
	res, err := d.Ranges(ctx, ranges, nil)
	if err != nil {
		return
	}

	resp = &spacesyncproto.HeadSyncResponse{
		Results: make([]*spacesyncproto.HeadSyncResult, 0, len(res)),
	}
	for _, rangeRes := range res {
		var elements []*spacesyncproto.HeadSyncResultElement
		if len(rangeRes.Elements) > 0 {
			elements = make([]*spacesyncproto.HeadSyncResultElement, 0, len(rangeRes.Elements))
			for _, el := range rangeRes.Elements {
				elements = append(elements, &spacesyncproto.HeadSyncResultElement{
					Id:   el.Id,
					Head: el.Head,
				})
			}
		}
		resp.Results = append(resp.Results, &spacesyncproto.HeadSyncResult{
			Hash:     rangeRes.Hash,
			Elements: elements,
			Count:    uint32(rangeRes.Count),
		})
	}
	resp.DiffType = d.DiffType()
	return
}
