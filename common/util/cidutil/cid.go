package cidutil

import (
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func NewCidFromBytes(data []byte) (string, error) {
	hash, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		return "", err
	}
	return cid.NewCidV1(cid.DagCBOR, hash).String(), nil
}

func VerifyCid(data []byte, id string) bool {
	hash, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		return false
	}
	return cid.NewCidV1(cid.DagCBOR, hash).String() == id
}
