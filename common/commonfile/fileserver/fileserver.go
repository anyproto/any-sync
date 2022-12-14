package fileserver

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/server"
)

const CName = "common.commonfile.fileservice"

func New() FileServer {
	return &fileServer{}
}

type FileServer interface {
	app.Component
}

type fileServer struct {
	store fileblockstore.BlockStore
}

func (f *fileServer) Init(a *app.App) (err error) {
	f.store = a.MustComponent(fileblockstore.CName).(fileblockstore.BlockStore)
	return fileproto.DRPCRegisterFile(a.MustComponent(server.CName).(server.DRPCServer), &rpcHandler{store: f.store})
}

func (f *fileServer) Name() (name string) {
	return CName
}
