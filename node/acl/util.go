package acl

import "github.com/ipfs/go-cid"

func cidToString(b []byte) (s string, err error) {
	rcid, err := cid.Cast(b)
	if err != nil {
		return
	}
	return rcid.String(), nil
}

func cidToByte(s string) (b []byte, err error) {
	rcid, err := cid.Decode(s)
	if err != nil {
		return
	}
	return rcid.Bytes(), nil
}
