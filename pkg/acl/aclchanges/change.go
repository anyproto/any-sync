package aclchanges

import (
	"github.com/gogo/protobuf/proto"
)

type Change interface {
	ProtoChange() proto.Marshaler
	DecryptedChangeContent() []byte
	Signature() []byte
	CID() string
}
