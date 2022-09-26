package aclrecordproto

import "github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"

func ACLReadKeyDerive(signKey []byte, encKey []byte) (*symmetric.Key, error) {
	concBuf := make([]byte, 0, len(signKey)+len(encKey))
	concBuf = append(concBuf, signKey...)
	concBuf = append(concBuf, encKey...)
	return symmetric.DeriveFromBytes(concBuf)
}
