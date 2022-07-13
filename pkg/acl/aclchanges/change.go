package aclchanges

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/pb"
)

type Change interface {
	ProtoChange() *pb.ACLChange
	DecryptedChangeContent() []byte
	Signature() []byte
	CID() string
}
