// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: nameservice/nameserviceproto/protos/nameservice.proto

package nameserviceproto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type NameAvailableRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Name including .any suffix
	FullName      string `protobuf:"bytes,1,opt,name=fullName,proto3" json:"fullName,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *NameAvailableRequest) Reset() {
	*x = NameAvailableRequest{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NameAvailableRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NameAvailableRequest) ProtoMessage() {}

func (x *NameAvailableRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NameAvailableRequest.ProtoReflect.Descriptor instead.
func (*NameAvailableRequest) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{0}
}

func (x *NameAvailableRequest) GetFullName() string {
	if x != nil {
		return x.FullName
	}
	return ""
}

type BatchNameAvailableRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Names including .any suffix
	FullNames     []string `protobuf:"bytes,1,rep,name=fullNames,proto3" json:"fullNames,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BatchNameAvailableRequest) Reset() {
	*x = BatchNameAvailableRequest{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BatchNameAvailableRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BatchNameAvailableRequest) ProtoMessage() {}

func (x *BatchNameAvailableRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BatchNameAvailableRequest.ProtoReflect.Descriptor instead.
func (*BatchNameAvailableRequest) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{1}
}

func (x *BatchNameAvailableRequest) GetFullNames() []string {
	if x != nil {
		return x.FullNames
	}
	return nil
}

type NameByAddressRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// EOA -> SCW -> name
	// A SCW Ethereum address that owns that name
	OwnerScwEthAddress string `protobuf:"bytes,1,opt,name=ownerScwEthAddress,proto3" json:"ownerScwEthAddress,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *NameByAddressRequest) Reset() {
	*x = NameByAddressRequest{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NameByAddressRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NameByAddressRequest) ProtoMessage() {}

func (x *NameByAddressRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NameByAddressRequest.ProtoReflect.Descriptor instead.
func (*NameByAddressRequest) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{2}
}

func (x *NameByAddressRequest) GetOwnerScwEthAddress() string {
	if x != nil {
		return x.OwnerScwEthAddress
	}
	return ""
}

type BatchNameByAddressRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// EOA -> SCW -> name
	// A SCW Ethereum address that owns that name
	OwnerScwEthAddresses []string `protobuf:"bytes,1,rep,name=ownerScwEthAddresses,proto3" json:"ownerScwEthAddresses,omitempty"`
	unknownFields        protoimpl.UnknownFields
	sizeCache            protoimpl.SizeCache
}

func (x *BatchNameByAddressRequest) Reset() {
	*x = BatchNameByAddressRequest{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BatchNameByAddressRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BatchNameByAddressRequest) ProtoMessage() {}

func (x *BatchNameByAddressRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BatchNameByAddressRequest.ProtoReflect.Descriptor instead.
func (*BatchNameByAddressRequest) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{3}
}

func (x *BatchNameByAddressRequest) GetOwnerScwEthAddresses() []string {
	if x != nil {
		return x.OwnerScwEthAddresses
	}
	return nil
}

type NameByAnyIdRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	AnyAddress    string                 `protobuf:"bytes,1,opt,name=anyAddress,proto3" json:"anyAddress,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *NameByAnyIdRequest) Reset() {
	*x = NameByAnyIdRequest{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NameByAnyIdRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NameByAnyIdRequest) ProtoMessage() {}

func (x *NameByAnyIdRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NameByAnyIdRequest.ProtoReflect.Descriptor instead.
func (*NameByAnyIdRequest) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{4}
}

func (x *NameByAnyIdRequest) GetAnyAddress() string {
	if x != nil {
		return x.AnyAddress
	}
	return ""
}

type BatchNameByAnyIdRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	AnyAddresses  []string               `protobuf:"bytes,1,rep,name=anyAddresses,proto3" json:"anyAddresses,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BatchNameByAnyIdRequest) Reset() {
	*x = BatchNameByAnyIdRequest{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BatchNameByAnyIdRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BatchNameByAnyIdRequest) ProtoMessage() {}

func (x *BatchNameByAnyIdRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BatchNameByAnyIdRequest.ProtoReflect.Descriptor instead.
func (*BatchNameByAnyIdRequest) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{5}
}

func (x *BatchNameByAnyIdRequest) GetAnyAddresses() []string {
	if x != nil {
		return x.AnyAddresses
	}
	return nil
}

type NameAvailableResponse struct {
	state     protoimpl.MessageState `protogen:"open.v1"`
	Available bool                   `protobuf:"varint,1,opt,name=available,proto3" json:"available,omitempty"`
	// EOA -> SCW -> name
	// This field is non-empty only if name is "already registered"
	OwnerScwEthAddress string `protobuf:"bytes,2,opt,name=ownerScwEthAddress,proto3" json:"ownerScwEthAddress,omitempty"`
	// This field is non-empty only if name is "already registered"
	OwnerEthAddress string `protobuf:"bytes,3,opt,name=ownerEthAddress,proto3" json:"ownerEthAddress,omitempty"`
	// A content hash attached to this name
	// This field is non-empty only if name is "already registered"
	OwnerAnyAddress string `protobuf:"bytes,4,opt,name=ownerAnyAddress,proto3" json:"ownerAnyAddress,omitempty"`
	// A SpaceID attached to this name
	// This field is non-empty only if name is "already registered"
	SpaceId string `protobuf:"bytes,5,opt,name=spaceId,proto3" json:"spaceId,omitempty"`
	// doesn't work with marashalling/unmarshalling
	// google.protobuf.Timestamp nameExpires = 5 [(gogoproto.stdtime) = true];
	NameExpires   int64 `protobuf:"varint,6,opt,name=nameExpires,proto3" json:"nameExpires,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *NameAvailableResponse) Reset() {
	*x = NameAvailableResponse{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NameAvailableResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NameAvailableResponse) ProtoMessage() {}

func (x *NameAvailableResponse) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NameAvailableResponse.ProtoReflect.Descriptor instead.
func (*NameAvailableResponse) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{6}
}

func (x *NameAvailableResponse) GetAvailable() bool {
	if x != nil {
		return x.Available
	}
	return false
}

func (x *NameAvailableResponse) GetOwnerScwEthAddress() string {
	if x != nil {
		return x.OwnerScwEthAddress
	}
	return ""
}

func (x *NameAvailableResponse) GetOwnerEthAddress() string {
	if x != nil {
		return x.OwnerEthAddress
	}
	return ""
}

func (x *NameAvailableResponse) GetOwnerAnyAddress() string {
	if x != nil {
		return x.OwnerAnyAddress
	}
	return ""
}

func (x *NameAvailableResponse) GetSpaceId() string {
	if x != nil {
		return x.SpaceId
	}
	return ""
}

func (x *NameAvailableResponse) GetNameExpires() int64 {
	if x != nil {
		return x.NameExpires
	}
	return 0
}

type BatchNameAvailableResponse struct {
	state         protoimpl.MessageState   `protogen:"open.v1"`
	Results       []*NameAvailableResponse `protobuf:"bytes,1,rep,name=results,proto3" json:"results,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BatchNameAvailableResponse) Reset() {
	*x = BatchNameAvailableResponse{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BatchNameAvailableResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BatchNameAvailableResponse) ProtoMessage() {}

func (x *BatchNameAvailableResponse) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BatchNameAvailableResponse.ProtoReflect.Descriptor instead.
func (*BatchNameAvailableResponse) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{7}
}

func (x *BatchNameAvailableResponse) GetResults() []*NameAvailableResponse {
	if x != nil {
		return x.Results
	}
	return nil
}

type NameByAddressResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Found         bool                   `protobuf:"varint,1,opt,name=found,proto3" json:"found,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *NameByAddressResponse) Reset() {
	*x = NameByAddressResponse{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NameByAddressResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NameByAddressResponse) ProtoMessage() {}

func (x *NameByAddressResponse) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NameByAddressResponse.ProtoReflect.Descriptor instead.
func (*NameByAddressResponse) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{8}
}

func (x *NameByAddressResponse) GetFound() bool {
	if x != nil {
		return x.Found
	}
	return false
}

func (x *NameByAddressResponse) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type BatchNameByAddressResponse struct {
	state         protoimpl.MessageState   `protogen:"open.v1"`
	Results       []*NameByAddressResponse `protobuf:"bytes,1,rep,name=results,proto3" json:"results,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BatchNameByAddressResponse) Reset() {
	*x = BatchNameByAddressResponse{}
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BatchNameByAddressResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BatchNameByAddressResponse) ProtoMessage() {}

func (x *BatchNameByAddressResponse) ProtoReflect() protoreflect.Message {
	mi := &file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BatchNameByAddressResponse.ProtoReflect.Descriptor instead.
func (*BatchNameByAddressResponse) Descriptor() ([]byte, []int) {
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP(), []int{9}
}

func (x *BatchNameByAddressResponse) GetResults() []*NameByAddressResponse {
	if x != nil {
		return x.Results
	}
	return nil
}

var File_nameservice_nameserviceproto_protos_nameservice_proto protoreflect.FileDescriptor

var file_nameservice_nameserviceproto_protos_nameservice_proto_rawDesc = string([]byte{
	0x0a, 0x35, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x6e, 0x61,
	0x6d, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2f, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x38, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x2f, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2f, 0x6e, 0x61, 0x6d,
	0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x61, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x32, 0x0a, 0x14, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62,
	0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x66, 0x75, 0x6c,
	0x6c, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x66, 0x75, 0x6c,
	0x6c, 0x4e, 0x61, 0x6d, 0x65, 0x22, 0x39, 0x0a, 0x19, 0x42, 0x61, 0x74, 0x63, 0x68, 0x4e, 0x61,
	0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x66, 0x75, 0x6c, 0x6c, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x18,
	0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x09, 0x66, 0x75, 0x6c, 0x6c, 0x4e, 0x61, 0x6d, 0x65, 0x73,
	0x22, 0x46, 0x0a, 0x14, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x2e, 0x0a, 0x12, 0x6f, 0x77, 0x6e, 0x65,
	0x72, 0x53, 0x63, 0x77, 0x45, 0x74, 0x68, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x12, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x53, 0x63, 0x77, 0x45, 0x74,
	0x68, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x22, 0x4f, 0x0a, 0x19, 0x42, 0x61, 0x74, 0x63,
	0x68, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x32, 0x0a, 0x14, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x53, 0x63,
	0x77, 0x45, 0x74, 0x68, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x65, 0x73, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x09, 0x52, 0x14, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x53, 0x63, 0x77, 0x45, 0x74, 0x68,
	0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x65, 0x73, 0x22, 0x34, 0x0a, 0x12, 0x4e, 0x61, 0x6d,
	0x65, 0x42, 0x79, 0x41, 0x6e, 0x79, 0x49, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x1e, 0x0a, 0x0a, 0x61, 0x6e, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0a, 0x61, 0x6e, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x22,
	0x3d, 0x0a, 0x17, 0x42, 0x61, 0x74, 0x63, 0x68, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x6e,
	0x79, 0x49, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x22, 0x0a, 0x0c, 0x61, 0x6e,
	0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x0c, 0x61, 0x6e, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x65, 0x73, 0x22, 0xf5,
	0x01, 0x0a, 0x15, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x61, 0x76, 0x61, 0x69,
	0x6c, 0x61, 0x62, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x61, 0x76, 0x61,
	0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x12, 0x2e, 0x0a, 0x12, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x53,
	0x63, 0x77, 0x45, 0x74, 0x68, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x12, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x53, 0x63, 0x77, 0x45, 0x74, 0x68, 0x41,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x28, 0x0a, 0x0f, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x45,
	0x74, 0x68, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0f, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x45, 0x74, 0x68, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	0x12, 0x28, 0x0a, 0x0f, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x41, 0x6e, 0x79, 0x41, 0x64, 0x64, 0x72,
	0x65, 0x73, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x6f, 0x77, 0x6e, 0x65, 0x72,
	0x41, 0x6e, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x70,
	0x61, 0x63, 0x65, 0x49, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73, 0x70, 0x61,
	0x63, 0x65, 0x49, 0x64, 0x12, 0x20, 0x0a, 0x0b, 0x6e, 0x61, 0x6d, 0x65, 0x45, 0x78, 0x70, 0x69,
	0x72, 0x65, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x6e, 0x61, 0x6d, 0x65, 0x45,
	0x78, 0x70, 0x69, 0x72, 0x65, 0x73, 0x22, 0x4e, 0x0a, 0x1a, 0x42, 0x61, 0x74, 0x63, 0x68, 0x4e,
	0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x30, 0x0a, 0x07, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x73, 0x18,
	0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69,
	0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52, 0x07, 0x72,
	0x65, 0x73, 0x75, 0x6c, 0x74, 0x73, 0x22, 0x41, 0x0a, 0x15, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79,
	0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x14, 0x0a, 0x05, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05,
	0x66, 0x6f, 0x75, 0x6e, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x4e, 0x0a, 0x1a, 0x42, 0x61, 0x74,
	0x63, 0x68, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x30, 0x0a, 0x07, 0x72, 0x65, 0x73, 0x75, 0x6c,
	0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x42,
	0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x52, 0x07, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x73, 0x32, 0xdc, 0x04, 0x0a, 0x05, 0x41, 0x6e,
	0x79, 0x6e, 0x73, 0x12, 0x42, 0x0a, 0x0f, 0x49, 0x73, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61,
	0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x12, 0x15, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61,
	0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e,
	0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x51, 0x0a, 0x14, 0x42, 0x61, 0x74, 0x63, 0x68,
	0x49, 0x73, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x12,
	0x1a, 0x2e, 0x42, 0x61, 0x74, 0x63, 0x68, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c,
	0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x42, 0x61,
	0x74, 0x63, 0x68, 0x4e, 0x61, 0x6d, 0x65, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x43, 0x0a, 0x10, 0x47, 0x65,
	0x74, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x15,
	0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x64,
	0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12,
	0x52, 0x0a, 0x15, 0x42, 0x61, 0x74, 0x63, 0x68, 0x47, 0x65, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x42,
	0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1a, 0x2e, 0x42, 0x61, 0x74, 0x63, 0x68,
	0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x42, 0x61, 0x74, 0x63, 0x68, 0x4e, 0x61, 0x6d, 0x65,
	0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x3f, 0x0a, 0x0e, 0x47, 0x65, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79,
	0x41, 0x6e, 0x79, 0x49, 0x64, 0x12, 0x13, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x6e,
	0x79, 0x49, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x4e, 0x61, 0x6d,
	0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x12, 0x4e, 0x0a, 0x13, 0x42, 0x61, 0x74, 0x63, 0x68, 0x47, 0x65, 0x74,
	0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x6e, 0x79, 0x49, 0x64, 0x12, 0x18, 0x2e, 0x42, 0x61,
	0x74, 0x63, 0x68, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x79, 0x41, 0x6e, 0x79, 0x49, 0x64, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x42, 0x61, 0x74, 0x63, 0x68, 0x4e, 0x61, 0x6d,
	0x65, 0x42, 0x79, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x12, 0x4b, 0x0a, 0x17, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x4e, 0x61, 0x6d,
	0x65, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x12,
	0x1a, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x1a, 0x12, 0x2e, 0x4f, 0x70,
	0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x45, 0x0a, 0x14, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x52, 0x65,
	0x6e, 0x65, 0x77, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x12, 0x17, 0x2e, 0x4e, 0x61, 0x6d, 0x65,
	0x52, 0x65, 0x6e, 0x65, 0x77, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x53, 0x69, 0x67, 0x6e,
	0x65, 0x64, 0x1a, 0x12, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x1e, 0x5a, 0x1c, 0x6e, 0x61, 0x6d, 0x65,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescOnce sync.Once
	file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescData []byte
)

func file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescGZIP() []byte {
	file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescOnce.Do(func() {
		file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_nameservice_nameserviceproto_protos_nameservice_proto_rawDesc), len(file_nameservice_nameserviceproto_protos_nameservice_proto_rawDesc)))
	})
	return file_nameservice_nameserviceproto_protos_nameservice_proto_rawDescData
}

var file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_nameservice_nameserviceproto_protos_nameservice_proto_goTypes = []any{
	(*NameAvailableRequest)(nil),       // 0: NameAvailableRequest
	(*BatchNameAvailableRequest)(nil),  // 1: BatchNameAvailableRequest
	(*NameByAddressRequest)(nil),       // 2: NameByAddressRequest
	(*BatchNameByAddressRequest)(nil),  // 3: BatchNameByAddressRequest
	(*NameByAnyIdRequest)(nil),         // 4: NameByAnyIdRequest
	(*BatchNameByAnyIdRequest)(nil),    // 5: BatchNameByAnyIdRequest
	(*NameAvailableResponse)(nil),      // 6: NameAvailableResponse
	(*BatchNameAvailableResponse)(nil), // 7: BatchNameAvailableResponse
	(*NameByAddressResponse)(nil),      // 8: NameByAddressResponse
	(*BatchNameByAddressResponse)(nil), // 9: BatchNameByAddressResponse
	(*NameRegisterRequestSigned)(nil),  // 10: NameRegisterRequestSigned
	(*NameRenewRequestSigned)(nil),     // 11: NameRenewRequestSigned
	(*OperationResponse)(nil),          // 12: OperationResponse
}
var file_nameservice_nameserviceproto_protos_nameservice_proto_depIdxs = []int32{
	6,  // 0: BatchNameAvailableResponse.results:type_name -> NameAvailableResponse
	8,  // 1: BatchNameByAddressResponse.results:type_name -> NameByAddressResponse
	0,  // 2: Anyns.IsNameAvailable:input_type -> NameAvailableRequest
	1,  // 3: Anyns.BatchIsNameAvailable:input_type -> BatchNameAvailableRequest
	2,  // 4: Anyns.GetNameByAddress:input_type -> NameByAddressRequest
	3,  // 5: Anyns.BatchGetNameByAddress:input_type -> BatchNameByAddressRequest
	4,  // 6: Anyns.GetNameByAnyId:input_type -> NameByAnyIdRequest
	5,  // 7: Anyns.BatchGetNameByAnyId:input_type -> BatchNameByAnyIdRequest
	10, // 8: Anyns.AdminNameRegisterSigned:input_type -> NameRegisterRequestSigned
	11, // 9: Anyns.AdminNameRenewSigned:input_type -> NameRenewRequestSigned
	6,  // 10: Anyns.IsNameAvailable:output_type -> NameAvailableResponse
	7,  // 11: Anyns.BatchIsNameAvailable:output_type -> BatchNameAvailableResponse
	8,  // 12: Anyns.GetNameByAddress:output_type -> NameByAddressResponse
	9,  // 13: Anyns.BatchGetNameByAddress:output_type -> BatchNameByAddressResponse
	8,  // 14: Anyns.GetNameByAnyId:output_type -> NameByAddressResponse
	9,  // 15: Anyns.BatchGetNameByAnyId:output_type -> BatchNameByAddressResponse
	12, // 16: Anyns.AdminNameRegisterSigned:output_type -> OperationResponse
	12, // 17: Anyns.AdminNameRenewSigned:output_type -> OperationResponse
	10, // [10:18] is the sub-list for method output_type
	2,  // [2:10] is the sub-list for method input_type
	2,  // [2:2] is the sub-list for extension type_name
	2,  // [2:2] is the sub-list for extension extendee
	0,  // [0:2] is the sub-list for field type_name
}

func init() { file_nameservice_nameserviceproto_protos_nameservice_proto_init() }
func file_nameservice_nameserviceproto_protos_nameservice_proto_init() {
	if File_nameservice_nameserviceproto_protos_nameservice_proto != nil {
		return
	}
	file_nameservice_nameserviceproto_protos_nameservice_aa_proto_init()
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_nameservice_nameserviceproto_protos_nameservice_proto_rawDesc), len(file_nameservice_nameserviceproto_protos_nameservice_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_nameservice_nameserviceproto_protos_nameservice_proto_goTypes,
		DependencyIndexes: file_nameservice_nameserviceproto_protos_nameservice_proto_depIdxs,
		MessageInfos:      file_nameservice_nameserviceproto_protos_nameservice_proto_msgTypes,
	}.Build()
	File_nameservice_nameserviceproto_protos_nameservice_proto = out.File
	file_nameservice_nameserviceproto_protos_nameservice_proto_goTypes = nil
	file_nameservice_nameserviceproto_protos_nameservice_proto_depIdxs = nil
}
