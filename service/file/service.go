package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/configuration"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/file/pogrebds"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool/handler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/mount"
	"github.com/ipfs/go-ipfs-blockstore"
	chunker "github.com/ipfs/go-ipfs-chunker"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipfs/go-unixfs/importer/trickle"
	ufsio "github.com/ipfs/go-unixfs/io"
	"github.com/multiformats/go-multihash"
	"go.uber.org/zap"
	"io"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
)

var log = logger.NewNamed("file")

const CName = "file"

func New() Service {
	return &service{}
}

type Service interface {
	app.ComponentRunnable
}

type service struct {
	pool pool.Pool
	conf configuration.Service

	blockstore  blockstore.Blockstore
	blocservice blockservice.BlockService
	exch        exchange.Interface
	merkledag   ipld.DAGService
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {

	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.conf = a.MustComponent(configuration.CName).(configuration.Service)
	ds, err := pogrebds.NewDatastore("/home/che/pogreb", &pogreb.Options{
		BackgroundSyncInterval: time.Minute,
	})
	if err != nil {
		return
	}
	mds := mount.New([]mount.Mount{
		{
			Prefix:    datastore.NewKey("/blocks"),
			Datastore: ds,
		},
	})

	s.blockstore = blockstore.NewBlockstore(mds)
	s.exch = &exch{s}
	s.blocservice = blockservice.New(s.blockstore, s.exch)
	s.merkledag = merkledag.NewDAGService(s.blocservice)
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	s.pool.AddHandler(syncproto.MessageType_MessageTypeFile, handler.Reply{ReplyHandler: s}.Handle)
	var addCount int64
	for i := 0; i < 8; i++ {
		go func() {
			ctx = context.Background()
			var bufbuf = make([]byte, 10*1024)
			for {
				buf := bufbuf[:rand.Intn(4*1024)+1024]
				rand.Read(buf)
				bb := bytes.NewBuffer(buf)
				n, e := s.AddFile(ctx, bb, nil)
				if e != nil {
					log.Error("AddFile error", zap.Error(e))
				}
				s.GetFile(ctx, n.Cid())
				atomic.AddInt64(&addCount, 1)
			}
		}()
	}
	go func() {
		var prev int64
		for {
			time.Sleep(time.Second)
			v := atomic.LoadInt64(&addCount)
			fmt.Println("add", v-prev)
			prev = v
		}
	}()
	return nil
}

func (s *service) GetBlock(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	blk, err := s.blockstore.Get(ctx, k)
	if ipld.IsNotFound(err) {
		return s.loadBlock(ctx, k)
	}
	return blk, err
}

func (s *service) GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error) {
	n, err := s.merkledag.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return ufsio.NewDagReader(ctx, n, s.merkledag)
}

// AddParams contains all of the configurable parameters needed to specify the
// importing process of a file.
type AddParams struct {
	Layout    string
	Chunker   string
	RawLeaves bool
	Hidden    bool
	Shard     bool
	NoCopy    bool
	HashFun   string
}

// AddFile chunks and adds content to the DAGService from a reader. The content
// is stored as a UnixFS DAG (default for IPFS). It returns the root
// ipld.Node.
func (s *service) AddFile(ctx context.Context, r io.Reader, params *AddParams) (ipld.Node, error) {
	if params == nil {
		params = &AddParams{}
	}
	if params.HashFun == "" {
		params.HashFun = "sha2-256"
	}

	prefix, err := merkledag.PrefixForCidVersion(1)
	if err != nil {
		return nil, fmt.Errorf("bad CID Version: %s", err)
	}

	hashFunCode, ok := multihash.Names[strings.ToLower(params.HashFun)]
	if !ok {
		return nil, fmt.Errorf("unrecognized hash function: %s", params.HashFun)
	}
	prefix.MhType = hashFunCode
	prefix.MhLength = -1

	dbp := helpers.DagBuilderParams{
		Dagserv:    s.merkledag,
		RawLeaves:  params.RawLeaves,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		NoCopy:     params.NoCopy,
		CidBuilder: &prefix,
	}

	chnk, err := chunker.FromString(r, params.Chunker)
	if err != nil {
		return nil, err
	}
	dbh, err := dbp.New(chnk)
	if err != nil {
		return nil, err
	}

	var n ipld.Node
	switch params.Layout {
	case "trickle":
		n, err = trickle.Layout(dbh)
	case "balanced", "":
		n, err = balanced.Layout(dbh)
	default:
		return nil, errors.New("invalid Layout")
	}
	return n, err
}

func (s *service) GetBlocks(ctx context.Context, ks []cid.Cid) (<-chan blocks.Block, error) {
	log.Info("GetBlocks")
	out := make(chan blocks.Block)
	go func() {
		// TODO: make queue
		defer close(out)
		for _, k := range ks {
			hit, err := s.GetBlock(ctx, k)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case out <- hit:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (s *service) NotifyNewBlocks(ctx context.Context, blocks ...blocks.Block) error {
	return nil
}

func (s *service) Handle(ctx context.Context, req []byte) (rep proto.Marshaler, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) loadBlock(ctx context.Context, k cid.Cid) (b blocks.Block, err error) {
	log.Info("load block", zap.String("cid", k.String()))
	return nil, fmt.Errorf("unable to load")
}

type exch struct {
	*service
}

func (e *exch) Close() error {
	return nil
}
