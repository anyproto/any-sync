package aclrecordproto

import (
	"github.com/anytypeio/any-sync/util/keys/symmetric"
)

func AclReadKeyDerive(signKey []byte, encKey []byte) (*symmetric.Key, error) {
	concBuf := make([]byte, 0, len(signKey)+len(encKey))
	concBuf = append(concBuf, signKey...)
	concBuf = append(concBuf, encKey...)
	return symmetric.DeriveFromBytes(concBuf)
}
