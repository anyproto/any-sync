package rpcstore

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpctest"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sort"
	"sync"
	"testing"
)

var ctx = context.Background()

func TestStore_Put(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)

	bs := []blocks.Block{
		blocks.NewBlock([]byte{'1'}),
		blocks.NewBlock([]byte{'2'}),
		blocks.NewBlock([]byte{'3'}),
	}
	err := fx.Add(ctx, bs)
	assert.NoError(t, err)
	for _, b := range bs {
		assert.NotNil(t, fx.serv.data[string(b.Cid().Bytes())])
	}

}

func TestStore_Delete(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)
	bs := []blocks.Block{
		blocks.NewBlock([]byte{'1'}),
	}
	err := fx.Add(ctx, bs)
	require.NoError(t, err)
	assert.Len(t, fx.serv.data, 1)
	require.NoError(t, fx.Delete(ctx, bs[0].Cid()))
	assert.Len(t, fx.serv.data, 0)
}

func TestStore_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)
		bs := []blocks.Block{
			blocks.NewBlock([]byte{'1'}),
		}
		err := fx.Add(ctx, bs)
		require.NoError(t, err)
		b, err := fx.Get(ctx, bs[0].Cid())
		require.NoError(t, err)
		assert.Equal(t, []byte{'1'}, b.RawData())
	})
	t.Run("not found", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.Finish(t)
		bs := []blocks.Block{
			blocks.NewBlock([]byte{'1'}),
		}
		b, err := fx.Get(ctx, bs[0].Cid())
		assert.Nil(t, b)
		assert.ErrorIs(t, err, fileblockstore.ErrCIDNotFound)
	})
}

func TestStore_GetMany(t *testing.T) {
	fx := newFixture(t)
	defer fx.Finish(t)

	bs := []blocks.Block{
		blocks.NewBlock([]byte{'1'}),
		blocks.NewBlock([]byte{'2'}),
		blocks.NewBlock([]byte{'3'}),
	}
	err := fx.Add(ctx, bs)
	assert.NoError(t, err)

	res := fx.GetMany(ctx, []cid.Cid{
		bs[0].Cid(),
		bs[1].Cid(),
		bs[2].Cid(),
	})
	var resBlocks []blocks.Block
	for b := range res {
		resBlocks = append(resBlocks, b)
	}
	require.Len(t, resBlocks, 3)
	sort.Slice(resBlocks, func(i, j int) bool {
		return string(resBlocks[i].RawData()) < string(resBlocks[j].RawData())
	})
	assert.Equal(t, bs, resBlocks)
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		a: new(app.App),
		s: New().(*service),
		serv: &testServer{
			data: make(map[string][]byte),
		},
	}

	conf := &config.Config{}

	for i := 0; i < 11; i++ {
		conf.Nodes = append(conf.Nodes, config.Node{
			PeerId: fmt.Sprint(i),
			Types:  []config.NodeType{config.NodeTypeFile},
		})
	}
	rserv := rpctest.NewTestServer()
	require.NoError(t, fileproto.DRPCRegisterFile(rserv.Mux, fx.serv))
	fx.a.Register(fx.s).
		Register(rpctest.NewTestPool().WithServer(rserv)).
		Register(nodeconf.New()).
		Register(conf)
	require.NoError(t, fx.a.Start(ctx))
	fx.store = fx.s.NewStore().(*store)
	return fx
}

type fixture struct {
	*store
	s    *service
	a    *app.App
	serv *testServer
}

func (fx *fixture) Finish(t *testing.T) {
	assert.NoError(t, fx.store.Close())
	assert.NoError(t, fx.a.Close(ctx))
}

type testServer struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (t *testServer) GetBlocks(stream fileproto.DRPCFile_GetBlocksStream) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		t.mu.Lock()
		resp := &fileproto.GetBlockResponse{
			Cid: req.Cid,
		}
		if data, ok := t.data[string(req.Cid)]; ok {
			resp.Data = data
		} else {
			resp.Code = fileproto.CIDError_CIDErrorNotFound
		}
		t.mu.Unlock()
		if err = stream.Send(resp); err != nil {
			return err
		}
	}
}

func (t *testServer) PushBlock(ctx context.Context, req *fileproto.PushBlockRequest) (*fileproto.PushBlockResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data[string(req.Cid)] = req.Data
	return &fileproto.PushBlockResponse{}, nil
}

func (t *testServer) DeleteBlocks(ctx context.Context, req *fileproto.DeleteBlocksRequest) (*fileproto.DeleteBlocksResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, c := range req.Cid {
		delete(t.data, string(c))
	}
	return &fileproto.DeleteBlocksResponse{}, nil
}

func (t *testServer) Check(ctx context.Context, req *fileproto.CheckRequest) (*fileproto.CheckResponse, error) {
	return &fileproto.CheckResponse{}, nil
}
