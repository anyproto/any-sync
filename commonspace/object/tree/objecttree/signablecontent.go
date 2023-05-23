package objecttree

import (
	"github.com/anyproto/any-sync/util/crypto"
)

// SignableChangeContent is a payload to be passed when we are creating change
type SignableChangeContent struct {
	// Data is a data provided by the client
	Data []byte
	// Key is the key which will be used to sign the change
	Key crypto.PrivKey
	// IsSnapshot tells if the change has snapshot of all previous data
	IsSnapshot bool
	// IsEncrypted tells if we encrypt the data with the relevant symmetric key
	IsEncrypted bool
	// Timestamp is a timestamp of change, if it is <= 0, then we use current timestamp
	Timestamp int64
}
