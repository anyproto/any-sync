// Code generated by protoc-gen-go-drpc. DO NOT EDIT.
// protoc-gen-go-drpc version: v0.0.32
// source: commonfile/fileproto/protos/file.proto

package fileproto

import (
	bytes "bytes"
	context "context"
	errors "errors"
	jsonpb "github.com/gogo/protobuf/jsonpb"
	proto "github.com/gogo/protobuf/proto"
	drpc "storj.io/drpc"
	drpcerr "storj.io/drpc/drpcerr"
)

type drpcEncoding_File_commonfile_fileproto_protos_file_proto struct{}

func (drpcEncoding_File_commonfile_fileproto_protos_file_proto) Marshal(msg drpc.Message) ([]byte, error) {
	return proto.Marshal(msg.(proto.Message))
}

func (drpcEncoding_File_commonfile_fileproto_protos_file_proto) Unmarshal(buf []byte, msg drpc.Message) error {
	return proto.Unmarshal(buf, msg.(proto.Message))
}

func (drpcEncoding_File_commonfile_fileproto_protos_file_proto) JSONMarshal(msg drpc.Message) ([]byte, error) {
	var buf bytes.Buffer
	err := new(jsonpb.Marshaler).Marshal(&buf, msg.(proto.Message))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (drpcEncoding_File_commonfile_fileproto_protos_file_proto) JSONUnmarshal(buf []byte, msg drpc.Message) error {
	return jsonpb.Unmarshal(bytes.NewReader(buf), msg.(proto.Message))
}

type DRPCFileClient interface {
	DRPCConn() drpc.Conn

	BlocksGet(ctx context.Context) (DRPCFile_BlocksGetClient, error)
	BlockPush(ctx context.Context, in *BlockPushRequest) (*BlockPushResponse, error)
	BlocksCheck(ctx context.Context, in *BlocksCheckRequest) (*BlocksCheckResponse, error)
	BlocksBind(ctx context.Context, in *BlocksBindRequest) (*BlocksBindResponse, error)
	BlocksDelete(ctx context.Context, in *BlocksDeleteRequest) (*BlocksDeleteResponse, error)
	Check(ctx context.Context, in *CheckRequest) (*CheckResponse, error)
}

type drpcFileClient struct {
	cc drpc.Conn
}

func NewDRPCFileClient(cc drpc.Conn) DRPCFileClient {
	return &drpcFileClient{cc}
}

func (c *drpcFileClient) DRPCConn() drpc.Conn { return c.cc }

func (c *drpcFileClient) BlocksGet(ctx context.Context) (DRPCFile_BlocksGetClient, error) {
	stream, err := c.cc.NewStream(ctx, "/filesync.File/BlocksGet", drpcEncoding_File_commonfile_fileproto_protos_file_proto{})
	if err != nil {
		return nil, err
	}
	x := &drpcFile_BlocksGetClient{stream}
	return x, nil
}

type DRPCFile_BlocksGetClient interface {
	drpc.Stream
	Send(*BlockGetRequest) error
	Recv() (*BlockGetResponse, error)
}

type drpcFile_BlocksGetClient struct {
	drpc.Stream
}

func (x *drpcFile_BlocksGetClient) Send(m *BlockGetRequest) error {
	return x.MsgSend(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{})
}

func (x *drpcFile_BlocksGetClient) Recv() (*BlockGetResponse, error) {
	m := new(BlockGetResponse)
	if err := x.MsgRecv(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{}); err != nil {
		return nil, err
	}
	return m, nil
}

func (x *drpcFile_BlocksGetClient) RecvMsg(m *BlockGetResponse) error {
	return x.MsgRecv(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{})
}

func (c *drpcFileClient) BlockPush(ctx context.Context, in *BlockPushRequest) (*BlockPushResponse, error) {
	out := new(BlockPushResponse)
	err := c.cc.Invoke(ctx, "/filesync.File/BlockPush", drpcEncoding_File_commonfile_fileproto_protos_file_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcFileClient) BlocksCheck(ctx context.Context, in *BlocksCheckRequest) (*BlocksCheckResponse, error) {
	out := new(BlocksCheckResponse)
	err := c.cc.Invoke(ctx, "/filesync.File/BlocksCheck", drpcEncoding_File_commonfile_fileproto_protos_file_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcFileClient) BlocksBind(ctx context.Context, in *BlocksBindRequest) (*BlocksBindResponse, error) {
	out := new(BlocksBindResponse)
	err := c.cc.Invoke(ctx, "/filesync.File/BlocksBind", drpcEncoding_File_commonfile_fileproto_protos_file_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcFileClient) BlocksDelete(ctx context.Context, in *BlocksDeleteRequest) (*BlocksDeleteResponse, error) {
	out := new(BlocksDeleteResponse)
	err := c.cc.Invoke(ctx, "/filesync.File/BlocksDelete", drpcEncoding_File_commonfile_fileproto_protos_file_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcFileClient) Check(ctx context.Context, in *CheckRequest) (*CheckResponse, error) {
	out := new(CheckResponse)
	err := c.cc.Invoke(ctx, "/filesync.File/Check", drpcEncoding_File_commonfile_fileproto_protos_file_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type DRPCFileServer interface {
	BlocksGet(DRPCFile_BlocksGetStream) error
	BlockPush(context.Context, *BlockPushRequest) (*BlockPushResponse, error)
	BlocksCheck(context.Context, *BlocksCheckRequest) (*BlocksCheckResponse, error)
	BlocksBind(context.Context, *BlocksBindRequest) (*BlocksBindResponse, error)
	BlocksDelete(context.Context, *BlocksDeleteRequest) (*BlocksDeleteResponse, error)
	Check(context.Context, *CheckRequest) (*CheckResponse, error)
}

type DRPCFileUnimplementedServer struct{}

func (s *DRPCFileUnimplementedServer) BlocksGet(DRPCFile_BlocksGetStream) error {
	return drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCFileUnimplementedServer) BlockPush(context.Context, *BlockPushRequest) (*BlockPushResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCFileUnimplementedServer) BlocksCheck(context.Context, *BlocksCheckRequest) (*BlocksCheckResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCFileUnimplementedServer) BlocksBind(context.Context, *BlocksBindRequest) (*BlocksBindResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCFileUnimplementedServer) BlocksDelete(context.Context, *BlocksDeleteRequest) (*BlocksDeleteResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCFileUnimplementedServer) Check(context.Context, *CheckRequest) (*CheckResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

type DRPCFileDescription struct{}

func (DRPCFileDescription) NumMethods() int { return 6 }

func (DRPCFileDescription) Method(n int) (string, drpc.Encoding, drpc.Receiver, interface{}, bool) {
	switch n {
	case 0:
		return "/filesync.File/BlocksGet", drpcEncoding_File_commonfile_fileproto_protos_file_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return nil, srv.(DRPCFileServer).
					BlocksGet(
						&drpcFile_BlocksGetStream{in1.(drpc.Stream)},
					)
			}, DRPCFileServer.BlocksGet, true
	case 1:
		return "/filesync.File/BlockPush", drpcEncoding_File_commonfile_fileproto_protos_file_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCFileServer).
					BlockPush(
						ctx,
						in1.(*BlockPushRequest),
					)
			}, DRPCFileServer.BlockPush, true
	case 2:
		return "/filesync.File/BlocksCheck", drpcEncoding_File_commonfile_fileproto_protos_file_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCFileServer).
					BlocksCheck(
						ctx,
						in1.(*BlocksCheckRequest),
					)
			}, DRPCFileServer.BlocksCheck, true
	case 3:
		return "/filesync.File/BlocksBind", drpcEncoding_File_commonfile_fileproto_protos_file_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCFileServer).
					BlocksBind(
						ctx,
						in1.(*BlocksBindRequest),
					)
			}, DRPCFileServer.BlocksBind, true
	case 4:
		return "/filesync.File/BlocksDelete", drpcEncoding_File_commonfile_fileproto_protos_file_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCFileServer).
					BlocksDelete(
						ctx,
						in1.(*BlocksDeleteRequest),
					)
			}, DRPCFileServer.BlocksDelete, true
	case 5:
		return "/filesync.File/Check", drpcEncoding_File_commonfile_fileproto_protos_file_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCFileServer).
					Check(
						ctx,
						in1.(*CheckRequest),
					)
			}, DRPCFileServer.Check, true
	default:
		return "", nil, nil, nil, false
	}
}

func DRPCRegisterFile(mux drpc.Mux, impl DRPCFileServer) error {
	return mux.Register(impl, DRPCFileDescription{})
}

type DRPCFile_BlocksGetStream interface {
	drpc.Stream
	Send(*BlockGetResponse) error
	Recv() (*BlockGetRequest, error)
}

type drpcFile_BlocksGetStream struct {
	drpc.Stream
}

func (x *drpcFile_BlocksGetStream) Send(m *BlockGetResponse) error {
	return x.MsgSend(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{})
}

func (x *drpcFile_BlocksGetStream) Recv() (*BlockGetRequest, error) {
	m := new(BlockGetRequest)
	if err := x.MsgRecv(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{}); err != nil {
		return nil, err
	}
	return m, nil
}

func (x *drpcFile_BlocksGetStream) RecvMsg(m *BlockGetRequest) error {
	return x.MsgRecv(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{})
}

type DRPCFile_BlockPushStream interface {
	drpc.Stream
	SendAndClose(*BlockPushResponse) error
}

type drpcFile_BlockPushStream struct {
	drpc.Stream
}

func (x *drpcFile_BlockPushStream) SendAndClose(m *BlockPushResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCFile_BlocksCheckStream interface {
	drpc.Stream
	SendAndClose(*BlocksCheckResponse) error
}

type drpcFile_BlocksCheckStream struct {
	drpc.Stream
}

func (x *drpcFile_BlocksCheckStream) SendAndClose(m *BlocksCheckResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCFile_BlocksBindStream interface {
	drpc.Stream
	SendAndClose(*BlocksBindResponse) error
}

type drpcFile_BlocksBindStream struct {
	drpc.Stream
}

func (x *drpcFile_BlocksBindStream) SendAndClose(m *BlocksBindResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCFile_BlocksDeleteStream interface {
	drpc.Stream
	SendAndClose(*BlocksDeleteResponse) error
}

type drpcFile_BlocksDeleteStream struct {
	drpc.Stream
}

func (x *drpcFile_BlocksDeleteStream) SendAndClose(m *BlocksDeleteResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCFile_CheckStream interface {
	drpc.Stream
	SendAndClose(*CheckResponse) error
}

type drpcFile_CheckStream struct {
	drpc.Stream
}

func (x *drpcFile_CheckStream) SendAndClose(m *CheckResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_commonfile_fileproto_protos_file_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}
