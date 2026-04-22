// Package listtest contains test-only helpers for the acl list package.
// Functions here may panic on error and must not be used outside tests.
package listtest

import (
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/cidutil"
)

// WrapAclRecord attaches a CID to the given raw record. It panics on
// marshal or CID errors and is intended for test code only.
func WrapAclRecord(rawRec *consensusproto.RawRecord) *consensusproto.RawRecordWithId {
	payload, err := rawRec.MarshalVT()
	if err != nil {
		panic(err)
	}
	id, err := cidutil.NewCidFromBytes(payload)
	if err != nil {
		panic(err)
	}
	return &consensusproto.RawRecordWithId{
		Payload: payload,
		Id:      id,
	}
}
