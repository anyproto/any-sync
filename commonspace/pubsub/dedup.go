package pubsub

import "sync"

const msgIdLen = 16

// msgIdDedup is a fixed-size duplicate-suppression cache keyed by message id.
// It is a FIFO ring: adding a new id evicts the oldest one, keeping memory flat.
type msgIdDedup struct {
	mu   sync.Mutex
	set  map[[msgIdLen]byte]struct{}
	ring [][msgIdLen]byte
	pos  int
	full bool
}

func newMsgIdDedup(size int) *msgIdDedup {
	return &msgIdDedup{
		set:  make(map[[msgIdLen]byte]struct{}, size),
		ring: make([][msgIdLen]byte, size),
	}
}

// seen reports whether id was already recorded; if not, it records it,
// evicting the oldest entry when the cache is full.
func (d *msgIdDedup) seen(id []byte) bool {
	if len(id) != msgIdLen {
		return false
	}
	var key [msgIdLen]byte
	copy(key[:], id)
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.set[key]; ok {
		return true
	}
	if d.full {
		delete(d.set, d.ring[d.pos])
	}
	d.ring[d.pos] = key
	d.set[key] = struct{}{}
	d.pos++
	if d.pos == len(d.ring) {
		d.pos = 0
		d.full = true
	}
	return false
}
