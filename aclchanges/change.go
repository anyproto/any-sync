package aclchanges

import "github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"

type Change interface {
	ProtoChange() *pb.ACLChange
	DecryptedChangeContent() []byte
	Signature() []byte
	CID() string
}
