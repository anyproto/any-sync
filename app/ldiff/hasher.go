package ldiff

import (
	"encoding/hex"

	"github.com/zeebo/blake3"
)

type Hasher struct {
	hasher *blake3.Hasher
}

func (h *Hasher) HashId(id string) string {
	h.hasher.Reset()
	h.hasher.WriteString(id)
	return hex.EncodeToString(h.hasher.Sum(nil))
}

func NewHasher() *Hasher {
	return &Hasher{hashersPool.Get().(*blake3.Hasher)}
}

func ReleaseHasher(hasher *Hasher) {
	hashersPool.Put(hasher.hasher)
}
