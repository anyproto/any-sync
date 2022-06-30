package threadmodels

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/fnv"

	"github.com/textileio/go-threads/core/thread"
)

func CreateACLThreadID(k SigningPubKey, docType uint16) (thread.ID, error) {
	rndlen := 32
	buf := make([]byte, 8+rndlen)

	// adding random bytes in the end
	_, err := rand.Read(buf[8 : 8+rndlen])
	if err != nil {
		panic("random read failed")
	}

	keyBytes, err := k.Bytes()
	if err != nil {
		return thread.Undef, err
	}

	hasher := fnv.New64()
	hasher.Write(keyBytes)
	res := hasher.Sum64()

	// putting hash of the pubkey in the beginning
	binary.LittleEndian.PutUint64(buf[:8], res)

	return threadIDFromBytes(docType, buf)
}

func VerifyACLThreadID(k SigningPubKey, threadId thread.ID) (bool, error) {
	bytes := threadId.Bytes()
	pubKeyBytes := threadId.Bytes()[len(bytes)-40 : len(bytes)-32]
	hash := binary.LittleEndian.Uint64(pubKeyBytes)

	keyBytes, err := k.Bytes()
	if err != nil {
		return false, err
	}

	hasher := fnv.New64()
	hasher.Write(keyBytes)
	realHash := hasher.Sum64()

	return hash == realHash, nil
}

func threadIDFromBytes(
	docType uint16,
	b []byte) (thread.ID, error) {
	blen := len(b)

	// two 8 bytes (max) numbers plus num
	buf := make([]byte, 2*binary.MaxVarintLen64+blen)
	n := binary.PutUvarint(buf, thread.V1)
	n += binary.PutUvarint(buf[n:], uint64(thread.AccessControlled))
	n += binary.PutUvarint(buf[n:], uint64(docType))

	cn := copy(buf[n:], b)
	if cn != blen {
		return thread.Undef, fmt.Errorf("copy length is inconsistent")
	}

	return thread.Cast(buf[:n+blen])
}
