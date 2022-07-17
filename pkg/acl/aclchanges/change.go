package aclchanges

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
)

type Change interface {
	ProtoChange() *aclpb.ACLChange
	DecryptedChangeContent() []byte
	Signature() []byte
	CID() string
}
