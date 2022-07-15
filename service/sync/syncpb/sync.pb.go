// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: service/sync/syncpb/protos/sync.proto

package syncpb

import (
	fmt "fmt"
	aclpb "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type Sync struct {
}

func (m *Sync) Reset()         { *m = Sync{} }
func (m *Sync) String() string { return proto.CompactTextString(m) }
func (*Sync) ProtoMessage()    {}
func (*Sync) Descriptor() ([]byte, []int) {
	return fileDescriptor_5f66cdd599c6466f, []int{0}
}
func (m *Sync) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Sync) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Sync.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Sync) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Sync.Merge(m, src)
}
func (m *Sync) XXX_Size() int {
	return m.Size()
}
func (m *Sync) XXX_DiscardUnknown() {
	xxx_messageInfo_Sync.DiscardUnknown(m)
}

var xxx_messageInfo_Sync proto.InternalMessageInfo

type SyncHeadUpdate struct {
	Heads   []*SyncHead        `protobuf:"bytes,1,rep,name=heads,proto3" json:"heads,omitempty"`
	Changes []*aclpb.RawChange `protobuf:"bytes,2,rep,name=changes,proto3" json:"changes,omitempty"`
	TreeId  string             `protobuf:"bytes,3,opt,name=treeId,proto3" json:"treeId,omitempty"`
}

func (m *SyncHeadUpdate) Reset()         { *m = SyncHeadUpdate{} }
func (m *SyncHeadUpdate) String() string { return proto.CompactTextString(m) }
func (*SyncHeadUpdate) ProtoMessage()    {}
func (*SyncHeadUpdate) Descriptor() ([]byte, []int) {
	return fileDescriptor_5f66cdd599c6466f, []int{0, 0}
}
func (m *SyncHeadUpdate) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SyncHeadUpdate) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SyncHeadUpdate.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SyncHeadUpdate) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SyncHeadUpdate.Merge(m, src)
}
func (m *SyncHeadUpdate) XXX_Size() int {
	return m.Size()
}
func (m *SyncHeadUpdate) XXX_DiscardUnknown() {
	xxx_messageInfo_SyncHeadUpdate.DiscardUnknown(m)
}

var xxx_messageInfo_SyncHeadUpdate proto.InternalMessageInfo

func (m *SyncHeadUpdate) GetHeads() []*SyncHead {
	if m != nil {
		return m.Heads
	}
	return nil
}

func (m *SyncHeadUpdate) GetChanges() []*aclpb.RawChange {
	if m != nil {
		return m.Changes
	}
	return nil
}

func (m *SyncHeadUpdate) GetTreeId() string {
	if m != nil {
		return m.TreeId
	}
	return ""
}

type SyncHead struct {
	Id           string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	SnapshotPath []string `protobuf:"bytes,2,rep,name=snapshotPath,proto3" json:"snapshotPath,omitempty"`
}

func (m *SyncHead) Reset()         { *m = SyncHead{} }
func (m *SyncHead) String() string { return proto.CompactTextString(m) }
func (*SyncHead) ProtoMessage()    {}
func (*SyncHead) Descriptor() ([]byte, []int) {
	return fileDescriptor_5f66cdd599c6466f, []int{0, 1}
}
func (m *SyncHead) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SyncHead) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SyncHead.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SyncHead) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SyncHead.Merge(m, src)
}
func (m *SyncHead) XXX_Size() int {
	return m.Size()
}
func (m *SyncHead) XXX_DiscardUnknown() {
	xxx_messageInfo_SyncHead.DiscardUnknown(m)
}

var xxx_messageInfo_SyncHead proto.InternalMessageInfo

func (m *SyncHead) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *SyncHead) GetSnapshotPath() []string {
	if m != nil {
		return m.SnapshotPath
	}
	return nil
}

type SyncFull struct {
}

func (m *SyncFull) Reset()         { *m = SyncFull{} }
func (m *SyncFull) String() string { return proto.CompactTextString(m) }
func (*SyncFull) ProtoMessage()    {}
func (*SyncFull) Descriptor() ([]byte, []int) {
	return fileDescriptor_5f66cdd599c6466f, []int{0, 2}
}
func (m *SyncFull) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SyncFull) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SyncFull.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SyncFull) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SyncFull.Merge(m, src)
}
func (m *SyncFull) XXX_Size() int {
	return m.Size()
}
func (m *SyncFull) XXX_DiscardUnknown() {
	xxx_messageInfo_SyncFull.DiscardUnknown(m)
}

var xxx_messageInfo_SyncFull proto.InternalMessageInfo

// here with send the request with all changes we have (we already know sender's snapshot path)
type SyncFullRequest struct {
	Heads   []*SyncHead        `protobuf:"bytes,1,rep,name=heads,proto3" json:"heads,omitempty"`
	Changes []*aclpb.RawChange `protobuf:"bytes,2,rep,name=changes,proto3" json:"changes,omitempty"`
	TreeId  string             `protobuf:"bytes,3,opt,name=treeId,proto3" json:"treeId,omitempty"`
}

func (m *SyncFullRequest) Reset()         { *m = SyncFullRequest{} }
func (m *SyncFullRequest) String() string { return proto.CompactTextString(m) }
func (*SyncFullRequest) ProtoMessage()    {}
func (*SyncFullRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5f66cdd599c6466f, []int{0, 2, 0}
}
func (m *SyncFullRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SyncFullRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SyncFullRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SyncFullRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SyncFullRequest.Merge(m, src)
}
func (m *SyncFullRequest) XXX_Size() int {
	return m.Size()
}
func (m *SyncFullRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_SyncFullRequest.DiscardUnknown(m)
}

var xxx_messageInfo_SyncFullRequest proto.InternalMessageInfo

func (m *SyncFullRequest) GetHeads() []*SyncHead {
	if m != nil {
		return m.Heads
	}
	return nil
}

func (m *SyncFullRequest) GetChanges() []*aclpb.RawChange {
	if m != nil {
		return m.Changes
	}
	return nil
}

func (m *SyncFullRequest) GetTreeId() string {
	if m != nil {
		return m.TreeId
	}
	return ""
}

type SyncFullResponse struct {
	Heads   []*SyncHead        `protobuf:"bytes,1,rep,name=heads,proto3" json:"heads,omitempty"`
	Changes []*aclpb.RawChange `protobuf:"bytes,2,rep,name=changes,proto3" json:"changes,omitempty"`
	TreeId  string             `protobuf:"bytes,3,opt,name=treeId,proto3" json:"treeId,omitempty"`
}

func (m *SyncFullResponse) Reset()         { *m = SyncFullResponse{} }
func (m *SyncFullResponse) String() string { return proto.CompactTextString(m) }
func (*SyncFullResponse) ProtoMessage()    {}
func (*SyncFullResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_5f66cdd599c6466f, []int{0, 2, 1}
}
func (m *SyncFullResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SyncFullResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SyncFullResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SyncFullResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SyncFullResponse.Merge(m, src)
}
func (m *SyncFullResponse) XXX_Size() int {
	return m.Size()
}
func (m *SyncFullResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_SyncFullResponse.DiscardUnknown(m)
}

var xxx_messageInfo_SyncFullResponse proto.InternalMessageInfo

func (m *SyncFullResponse) GetHeads() []*SyncHead {
	if m != nil {
		return m.Heads
	}
	return nil
}

func (m *SyncFullResponse) GetChanges() []*aclpb.RawChange {
	if m != nil {
		return m.Changes
	}
	return nil
}

func (m *SyncFullResponse) GetTreeId() string {
	if m != nil {
		return m.TreeId
	}
	return ""
}

func init() {
	proto.RegisterType((*Sync)(nil), "anytype.Sync")
	proto.RegisterType((*SyncHeadUpdate)(nil), "anytype.Sync.HeadUpdate")
	proto.RegisterType((*SyncHead)(nil), "anytype.Sync.Head")
	proto.RegisterType((*SyncFull)(nil), "anytype.Sync.Full")
	proto.RegisterType((*SyncFullRequest)(nil), "anytype.Sync.Full.Request")
	proto.RegisterType((*SyncFullResponse)(nil), "anytype.Sync.Full.Response")
}

func init() {
	proto.RegisterFile("service/sync/syncpb/protos/sync.proto", fileDescriptor_5f66cdd599c6466f)
}

var fileDescriptor_5f66cdd599c6466f = []byte{
	// 297 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xbc, 0x92, 0xb1, 0x4e, 0xc3, 0x30,
	0x10, 0x86, 0xeb, 0xb6, 0xb4, 0xf4, 0x40, 0x1d, 0x3c, 0xa0, 0x28, 0x83, 0x55, 0x55, 0x42, 0xca,
	0xe4, 0x22, 0xd8, 0x18, 0x41, 0x42, 0xb0, 0x21, 0x23, 0x16, 0x36, 0xd7, 0x3e, 0x35, 0x15, 0x91,
	0x63, 0x6a, 0xb7, 0x90, 0xb7, 0xe0, 0x61, 0x78, 0x08, 0xc6, 0x8e, 0x8c, 0x28, 0x79, 0x06, 0x76,
	0x14, 0x27, 0xa8, 0xe2, 0x05, 0x3a, 0xd8, 0xba, 0xfb, 0xef, 0xbb, 0xdf, 0x96, 0x7d, 0x70, 0xea,
	0x70, 0xb5, 0x59, 0x2a, 0x9c, 0xb9, 0xc2, 0xa8, 0xb0, 0xd9, 0xf9, 0xcc, 0xae, 0x72, 0x9f, 0xbb,
	0x90, 0xf1, 0x10, 0xd3, 0xa1, 0x34, 0x85, 0x2f, 0x2c, 0xc6, 0x67, 0xf6, 0x79, 0x31, 0x93, 0x2a,
	0xab, 0x97, 0x4a, 0xa5, 0x59, 0xa0, 0xab, 0xc3, 0x5d, 0xd3, 0x4e, 0x6f, 0x5a, 0xa7, 0x1f, 0x3d,
	0xe8, 0x3f, 0x14, 0x46, 0xc5, 0x6f, 0x00, 0xb7, 0x28, 0xf5, 0xa3, 0xd5, 0xd2, 0x23, 0x4d, 0xe0,
	0x20, 0x45, 0xa9, 0x5d, 0x44, 0x26, 0xbd, 0xe4, 0xe8, 0x9c, 0xf2, 0xf6, 0x04, 0x5e, 0xb3, 0xbc,
	0x06, 0x45, 0x03, 0xd0, 0x04, 0x86, 0xad, 0x63, 0xd4, 0x0d, 0xec, 0x98, 0x4b, 0x95, 0x71, 0x21,
	0x5f, 0xaf, 0x83, 0x2c, 0xfe, 0xca, 0xf4, 0x04, 0x06, 0x7e, 0x85, 0x78, 0xa7, 0xa3, 0xde, 0x84,
	0x24, 0x23, 0xd1, 0x66, 0xf1, 0x25, 0xf4, 0x6b, 0x43, 0x3a, 0x86, 0xee, 0x52, 0x47, 0x24, 0xd4,
	0xba, 0x4b, 0x4d, 0xa7, 0x70, 0xec, 0x8c, 0xb4, 0x2e, 0xcd, 0xfd, 0xbd, 0xf4, 0x69, 0xb0, 0x1f,
	0x89, 0x7f, 0x5a, 0xfc, 0x43, 0xa0, 0x7f, 0xb3, 0xce, 0xb2, 0x78, 0x0d, 0x43, 0x81, 0x2f, 0x6b,
	0x74, 0x7e, 0xaf, 0x77, 0xdf, 0xc0, 0xa1, 0x40, 0x67, 0x73, 0xe3, 0xf6, 0xfa, 0x66, 0x57, 0x93,
	0xcf, 0x92, 0x91, 0x6d, 0xc9, 0xc8, 0x77, 0xc9, 0xc8, 0x7b, 0xc5, 0x3a, 0xdb, 0x8a, 0x75, 0xbe,
	0x2a, 0xd6, 0x79, 0x1a, 0x34, 0x53, 0x32, 0x1f, 0x84, 0xff, 0xbd, 0xf8, 0x0d, 0x00, 0x00, 0xff,
	0xff, 0xac, 0xf4, 0x50, 0xb7, 0x43, 0x02, 0x00, 0x00,
}

func (m *Sync) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Sync) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Sync) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func (m *SyncHeadUpdate) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SyncHeadUpdate) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SyncHeadUpdate) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.TreeId) > 0 {
		i -= len(m.TreeId)
		copy(dAtA[i:], m.TreeId)
		i = encodeVarintSync(dAtA, i, uint64(len(m.TreeId)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Changes) > 0 {
		for iNdEx := len(m.Changes) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Changes[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSync(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Heads) > 0 {
		for iNdEx := len(m.Heads) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Heads[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSync(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *SyncHead) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SyncHead) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SyncHead) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.SnapshotPath) > 0 {
		for iNdEx := len(m.SnapshotPath) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.SnapshotPath[iNdEx])
			copy(dAtA[i:], m.SnapshotPath[iNdEx])
			i = encodeVarintSync(dAtA, i, uint64(len(m.SnapshotPath[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Id) > 0 {
		i -= len(m.Id)
		copy(dAtA[i:], m.Id)
		i = encodeVarintSync(dAtA, i, uint64(len(m.Id)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *SyncFull) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SyncFull) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SyncFull) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func (m *SyncFullRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SyncFullRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SyncFullRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.TreeId) > 0 {
		i -= len(m.TreeId)
		copy(dAtA[i:], m.TreeId)
		i = encodeVarintSync(dAtA, i, uint64(len(m.TreeId)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Changes) > 0 {
		for iNdEx := len(m.Changes) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Changes[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSync(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Heads) > 0 {
		for iNdEx := len(m.Heads) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Heads[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSync(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *SyncFullResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SyncFullResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SyncFullResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.TreeId) > 0 {
		i -= len(m.TreeId)
		copy(dAtA[i:], m.TreeId)
		i = encodeVarintSync(dAtA, i, uint64(len(m.TreeId)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Changes) > 0 {
		for iNdEx := len(m.Changes) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Changes[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSync(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Heads) > 0 {
		for iNdEx := len(m.Heads) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Heads[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSync(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintSync(dAtA []byte, offset int, v uint64) int {
	offset -= sovSync(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Sync) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func (m *SyncHeadUpdate) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Heads) > 0 {
		for _, e := range m.Heads {
			l = e.Size()
			n += 1 + l + sovSync(uint64(l))
		}
	}
	if len(m.Changes) > 0 {
		for _, e := range m.Changes {
			l = e.Size()
			n += 1 + l + sovSync(uint64(l))
		}
	}
	l = len(m.TreeId)
	if l > 0 {
		n += 1 + l + sovSync(uint64(l))
	}
	return n
}

func (m *SyncHead) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Id)
	if l > 0 {
		n += 1 + l + sovSync(uint64(l))
	}
	if len(m.SnapshotPath) > 0 {
		for _, s := range m.SnapshotPath {
			l = len(s)
			n += 1 + l + sovSync(uint64(l))
		}
	}
	return n
}

func (m *SyncFull) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func (m *SyncFullRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Heads) > 0 {
		for _, e := range m.Heads {
			l = e.Size()
			n += 1 + l + sovSync(uint64(l))
		}
	}
	if len(m.Changes) > 0 {
		for _, e := range m.Changes {
			l = e.Size()
			n += 1 + l + sovSync(uint64(l))
		}
	}
	l = len(m.TreeId)
	if l > 0 {
		n += 1 + l + sovSync(uint64(l))
	}
	return n
}

func (m *SyncFullResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Heads) > 0 {
		for _, e := range m.Heads {
			l = e.Size()
			n += 1 + l + sovSync(uint64(l))
		}
	}
	if len(m.Changes) > 0 {
		for _, e := range m.Changes {
			l = e.Size()
			n += 1 + l + sovSync(uint64(l))
		}
	}
	l = len(m.TreeId)
	if l > 0 {
		n += 1 + l + sovSync(uint64(l))
	}
	return n
}

func sovSync(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozSync(x uint64) (n int) {
	return sovSync(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Sync) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSync
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Sync: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Sync: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipSync(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSync
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SyncHeadUpdate) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSync
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: HeadUpdate: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HeadUpdate: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Heads", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Heads = append(m.Heads, &SyncHead{})
			if err := m.Heads[len(m.Heads)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Changes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Changes = append(m.Changes, &aclpb.RawChange{})
			if err := m.Changes[len(m.Changes)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TreeId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TreeId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSync(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSync
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SyncHead) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSync
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Head: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Head: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Id", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Id = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SnapshotPath", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SnapshotPath = append(m.SnapshotPath, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSync(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSync
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SyncFull) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSync
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Full: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Full: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipSync(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSync
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SyncFullRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSync
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Request: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Request: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Heads", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Heads = append(m.Heads, &SyncHead{})
			if err := m.Heads[len(m.Heads)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Changes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Changes = append(m.Changes, &aclpb.RawChange{})
			if err := m.Changes[len(m.Changes)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TreeId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TreeId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSync(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSync
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SyncFullResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSync
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Response: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Response: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Heads", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Heads = append(m.Heads, &SyncHead{})
			if err := m.Heads[len(m.Heads)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Changes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Changes = append(m.Changes, &aclpb.RawChange{})
			if err := m.Changes[len(m.Changes)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TreeId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSync
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthSync
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSync
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TreeId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSync(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSync
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipSync(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowSync
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowSync
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowSync
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthSync
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupSync
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthSync
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthSync        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowSync          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupSync = fmt.Errorf("proto: unexpected end of group")
)
