// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: commonspace/sync/synctestproto/protos/synctest.proto

package synctestproto

import (
	fmt "fmt"
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

type CounterIncrease struct {
	Value    int32  `protobuf:"varint,1,opt,name=value,proto3" json:"value,omitempty"`
	ObjectId string `protobuf:"bytes,2,opt,name=objectId,proto3" json:"objectId,omitempty"`
}

func (m *CounterIncrease) Reset()         { *m = CounterIncrease{} }
func (m *CounterIncrease) String() string { return proto.CompactTextString(m) }
func (*CounterIncrease) ProtoMessage()    {}
func (*CounterIncrease) Descriptor() ([]byte, []int) {
	return fileDescriptor_dd5c22b15d7f69e4, []int{0}
}
func (m *CounterIncrease) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CounterIncrease) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CounterIncrease.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CounterIncrease) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CounterIncrease.Merge(m, src)
}
func (m *CounterIncrease) XXX_Size() int {
	return m.Size()
}
func (m *CounterIncrease) XXX_DiscardUnknown() {
	xxx_messageInfo_CounterIncrease.DiscardUnknown(m)
}

var xxx_messageInfo_CounterIncrease proto.InternalMessageInfo

func (m *CounterIncrease) GetValue() int32 {
	if m != nil {
		return m.Value
	}
	return 0
}

func (m *CounterIncrease) GetObjectId() string {
	if m != nil {
		return m.ObjectId
	}
	return ""
}

type CounterRequest struct {
	ExistingValues []int32 `protobuf:"varint,1,rep,packed,name=existingValues,proto3" json:"existingValues,omitempty"`
	ObjectId       string  `protobuf:"bytes,2,opt,name=objectId,proto3" json:"objectId,omitempty"`
}

func (m *CounterRequest) Reset()         { *m = CounterRequest{} }
func (m *CounterRequest) String() string { return proto.CompactTextString(m) }
func (*CounterRequest) ProtoMessage()    {}
func (*CounterRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_dd5c22b15d7f69e4, []int{1}
}
func (m *CounterRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CounterRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CounterRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CounterRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CounterRequest.Merge(m, src)
}
func (m *CounterRequest) XXX_Size() int {
	return m.Size()
}
func (m *CounterRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CounterRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CounterRequest proto.InternalMessageInfo

func (m *CounterRequest) GetExistingValues() []int32 {
	if m != nil {
		return m.ExistingValues
	}
	return nil
}

func (m *CounterRequest) GetObjectId() string {
	if m != nil {
		return m.ObjectId
	}
	return ""
}

func init() {
	proto.RegisterType((*CounterIncrease)(nil), "synctest.CounterIncrease")
	proto.RegisterType((*CounterRequest)(nil), "synctest.CounterRequest")
}

func init() {
	proto.RegisterFile("commonspace/sync/synctestproto/protos/synctest.proto", fileDescriptor_dd5c22b15d7f69e4)
}

var fileDescriptor_dd5c22b15d7f69e4 = []byte{
	// 254 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x32, 0x49, 0xce, 0xcf, 0xcd,
	0xcd, 0xcf, 0x2b, 0x2e, 0x48, 0x4c, 0x4e, 0xd5, 0x2f, 0xae, 0xcc, 0x4b, 0x06, 0x13, 0x25, 0xa9,
	0xc5, 0x25, 0x05, 0x45, 0xf9, 0x25, 0xf9, 0xfa, 0x60, 0xb2, 0x18, 0x2e, 0xa8, 0x07, 0xe6, 0x0b,
	0x71, 0xc0, 0xf8, 0x4a, 0xce, 0x5c, 0xfc, 0xce, 0xf9, 0xa5, 0x79, 0x25, 0xa9, 0x45, 0x9e, 0x79,
	0xc9, 0x45, 0xa9, 0x89, 0xc5, 0xa9, 0x42, 0x22, 0x5c, 0xac, 0x65, 0x89, 0x39, 0xa5, 0xa9, 0x12,
	0x8c, 0x0a, 0x8c, 0x1a, 0xac, 0x41, 0x10, 0x8e, 0x90, 0x14, 0x17, 0x47, 0x7e, 0x52, 0x56, 0x6a,
	0x72, 0x89, 0x67, 0x8a, 0x04, 0x93, 0x02, 0xa3, 0x06, 0x67, 0x10, 0x9c, 0xaf, 0x14, 0xc2, 0xc5,
	0x07, 0x35, 0x24, 0x28, 0xb5, 0xb0, 0x34, 0xb5, 0xb8, 0x44, 0x48, 0x8d, 0x8b, 0x2f, 0xb5, 0x22,
	0xb3, 0xb8, 0x24, 0x33, 0x2f, 0x3d, 0x0c, 0xa4, 0xbd, 0x58, 0x82, 0x51, 0x81, 0x59, 0x83, 0x35,
	0x08, 0x4d, 0x14, 0x9f, 0xa9, 0x46, 0xcb, 0x19, 0xb9, 0xb8, 0xa1, 0xc6, 0x06, 0x57, 0xe6, 0x25,
	0x0b, 0xf9, 0x72, 0x89, 0xc0, 0xb8, 0x25, 0x45, 0xa9, 0x89, 0xb9, 0x30, 0xbb, 0x24, 0xf4, 0xe0,
	0xbe, 0x43, 0x75, 0x85, 0x94, 0x24, 0x86, 0x0c, 0xcc, 0x93, 0x06, 0x8c, 0x42, 0x9e, 0x5c, 0xbc,
	0x28, 0xc6, 0x09, 0xe1, 0x56, 0x8d, 0xc7, 0x20, 0x0d, 0x46, 0x03, 0x46, 0x27, 0x8b, 0x13, 0x8f,
	0xe4, 0x18, 0x2f, 0x3c, 0x92, 0x63, 0x7c, 0xf0, 0x48, 0x8e, 0x71, 0xc2, 0x63, 0x39, 0x86, 0x0b,
	0x8f, 0xe5, 0x18, 0x6e, 0x3c, 0x96, 0x63, 0x88, 0x92, 0xc3, 0x1f, 0x3d, 0x49, 0x6c, 0x60, 0xca,
	0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0x5b, 0xf3, 0x0f, 0x8d, 0xc7, 0x01, 0x00, 0x00,
}

func (m *CounterIncrease) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CounterIncrease) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *CounterIncrease) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.ObjectId) > 0 {
		i -= len(m.ObjectId)
		copy(dAtA[i:], m.ObjectId)
		i = encodeVarintSynctest(dAtA, i, uint64(len(m.ObjectId)))
		i--
		dAtA[i] = 0x12
	}
	if m.Value != 0 {
		i = encodeVarintSynctest(dAtA, i, uint64(m.Value))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *CounterRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CounterRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *CounterRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.ObjectId) > 0 {
		i -= len(m.ObjectId)
		copy(dAtA[i:], m.ObjectId)
		i = encodeVarintSynctest(dAtA, i, uint64(len(m.ObjectId)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.ExistingValues) > 0 {
		dAtA2 := make([]byte, len(m.ExistingValues)*10)
		var j1 int
		for _, num1 := range m.ExistingValues {
			num := uint64(num1)
			for num >= 1<<7 {
				dAtA2[j1] = uint8(uint64(num)&0x7f | 0x80)
				num >>= 7
				j1++
			}
			dAtA2[j1] = uint8(num)
			j1++
		}
		i -= j1
		copy(dAtA[i:], dAtA2[:j1])
		i = encodeVarintSynctest(dAtA, i, uint64(j1))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintSynctest(dAtA []byte, offset int, v uint64) int {
	offset -= sovSynctest(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *CounterIncrease) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Value != 0 {
		n += 1 + sovSynctest(uint64(m.Value))
	}
	l = len(m.ObjectId)
	if l > 0 {
		n += 1 + l + sovSynctest(uint64(l))
	}
	return n
}

func (m *CounterRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.ExistingValues) > 0 {
		l = 0
		for _, e := range m.ExistingValues {
			l += sovSynctest(uint64(e))
		}
		n += 1 + sovSynctest(uint64(l)) + l
	}
	l = len(m.ObjectId)
	if l > 0 {
		n += 1 + l + sovSynctest(uint64(l))
	}
	return n
}

func sovSynctest(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozSynctest(x uint64) (n int) {
	return sovSynctest(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *CounterIncrease) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSynctest
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
			return fmt.Errorf("proto: CounterIncrease: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: CounterIncrease: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			m.Value = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSynctest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Value |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ObjectId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSynctest
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
				return ErrInvalidLengthSynctest
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSynctest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ObjectId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSynctest(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSynctest
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
func (m *CounterRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSynctest
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
			return fmt.Errorf("proto: CounterRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: CounterRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType == 0 {
				var v int32
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowSynctest
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					v |= int32(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				m.ExistingValues = append(m.ExistingValues, v)
			} else if wireType == 2 {
				var packedLen int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowSynctest
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					packedLen |= int(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if packedLen < 0 {
					return ErrInvalidLengthSynctest
				}
				postIndex := iNdEx + packedLen
				if postIndex < 0 {
					return ErrInvalidLengthSynctest
				}
				if postIndex > l {
					return io.ErrUnexpectedEOF
				}
				var elementCount int
				var count int
				for _, integer := range dAtA[iNdEx:postIndex] {
					if integer < 128 {
						count++
					}
				}
				elementCount = count
				if elementCount != 0 && len(m.ExistingValues) == 0 {
					m.ExistingValues = make([]int32, 0, elementCount)
				}
				for iNdEx < postIndex {
					var v int32
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowSynctest
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						v |= int32(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					m.ExistingValues = append(m.ExistingValues, v)
				}
			} else {
				return fmt.Errorf("proto: wrong wireType = %d for field ExistingValues", wireType)
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ObjectId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSynctest
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
				return ErrInvalidLengthSynctest
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSynctest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ObjectId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSynctest(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSynctest
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
func skipSynctest(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowSynctest
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
					return 0, ErrIntOverflowSynctest
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
					return 0, ErrIntOverflowSynctest
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
				return 0, ErrInvalidLengthSynctest
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupSynctest
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthSynctest
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthSynctest        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowSynctest          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupSynctest = fmt.Errorf("proto: unexpected end of group")
)