package badgerfilestore

import (
	"bytes"
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/ipfs/go-cid"
	"sync"
	"time"
)

const keyIndexPrefix = "files/indexes/"

type Op string

const (
	OpAdd    Op = "add"
	OpDelete Op = "del"
	OpLoad   Op = "load"
)

func NewFileBadgerIndex(db *badger.DB) *FileBadgerIndex {
	return &FileBadgerIndex{
		db:     db,
		workCh: make(chan struct{}, 1),
	}
}

var cidsPool = &sync.Pool{
	New: func() any {
		return &Cids{}
	},
}

type FileBadgerIndex struct {
	db     *badger.DB
	workCh chan struct{}
}

func (i *FileBadgerIndex) Add(cids *Cids) error {
	addTimeBin, _ := time.Now().MarshalBinary()
	defer i.pingWorkCh()
	return i.db.Update(func(txn *badger.Txn) error {
		keys := cids.Keys()
		for _, k := range keys {
			if err := txn.Set(k, addTimeBin); err != nil {
				return err
			}
		}
		return nil
	})
}

func (i *FileBadgerIndex) Done(cids *Cids) (err error) {
	if len(cids.SpaceOps) == 0 {
		return nil
	}
	return i.db.Update(func(txn *badger.Txn) error {
		keys := cids.Keys()
		for _, k := range keys {
			if err := txn.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}
func (i *FileBadgerIndex) List(limit int) (cids *Cids, err error) {
	cids = NewCids()
	err = i.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchSize:   limit,
			PrefetchValues: false,
			Prefix:         []byte(keyIndexPrefix),
		})
		defer it.Close()
		var l int
		for it.Rewind(); it.Valid(); it.Next() {
			e := cids.AddKey(it.Item().Key())
			if e == nil {
				l++
				if l == limit {
					return nil
				}
			}
		}
		return nil
	})
	return
}

func (i *FileBadgerIndex) Len() (l int, err error) {
	err = i.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchSize:   100,
			PrefetchValues: false,
			Prefix:         []byte(keyIndexPrefix),
		})
		defer it.Close()
		var l int
		for it.Rewind(); it.Valid(); it.Next() {
			l++
		}
		return nil
	})
	return
}

func (i *FileBadgerIndex) pingWorkCh() {
	select {
	case i.workCh <- struct{}{}:
	default:
	}
}

func (i *FileBadgerIndex) HasWorkCh() chan struct{} {
	l, err := i.Len()
	if err != nil {
		return i.workCh
	}
	if l > 0 {
		i.pingWorkCh()
	}
	return i.workCh
}

var errInvalidKey = errors.New("invalid key")

var sep = []byte("/")

func parseCIDOp(key []byte) (spaceId string, op Op, k cid.Cid, err error) {
	if len(key) <= len(keyIndexPrefix) {
		err = errInvalidKey
		return
	}
	key = key[len(keyIndexPrefix):]
	fi := bytes.Index(key, sep)
	if fi < 0 {
		err = errInvalidKey
		return
	}
	spaceId = string(key[:fi])
	key = key[fi+1:]
	fi = bytes.Index(key, sep)
	if fi < 0 {
		err = errInvalidKey
		return
	}
	op = Op(key[:fi])
	k, err = cid.Cast(key[fi+1:])
	return
}

func NewCids() *Cids {
	return cidsPool.Get().(*Cids)
}

type SpaceCidOps struct {
	SpaceId string
	Add     []cid.Cid
	Delete  []cid.Cid
	Load    []cid.Cid
}

type Cids struct {
	SpaceOps []SpaceCidOps
	keysBuf  [][]byte
}

func (c *Cids) Add(spaceId string, op Op, k cid.Cid) {
	var spaceIndex = -1
	for i, so := range c.SpaceOps {
		if so.SpaceId == spaceId {
			spaceIndex = i
			break
		}
	}
	if spaceIndex == -1 {
		spaceIndex = len(c.SpaceOps)
		if len(c.SpaceOps) < cap(c.SpaceOps) {
			c.SpaceOps = c.SpaceOps[0 : len(c.SpaceOps)+1]
			c.SpaceOps[spaceIndex].SpaceId = spaceId
		} else {
			c.SpaceOps = append(c.SpaceOps, SpaceCidOps{SpaceId: spaceId})
		}
	}
	switch op {
	case OpAdd:
		c.SpaceOps[spaceIndex].Add = append(c.SpaceOps[spaceIndex].Add, k)
	case OpDelete:
		c.SpaceOps[spaceIndex].Delete = append(c.SpaceOps[spaceIndex].Delete, k)
	case OpLoad:
		c.SpaceOps[spaceIndex].Load = append(c.SpaceOps[spaceIndex].Load, k)
	}
}

func (c *Cids) Keys() [][]byte {
	addKey := func(spaceId string, k cid.Cid, op Op) {
		if len(c.keysBuf) < cap(c.keysBuf) {
			c.keysBuf = c.keysBuf[:len(c.keysBuf)+1]
		} else {
			c.keysBuf = append(c.keysBuf, nil)
		}
		ki := len(c.keysBuf) - 1
		buf := bytes.NewBuffer(c.keysBuf[ki][:0])
		buf.WriteString(keyIndexPrefix)
		buf.WriteString(spaceId)
		buf.WriteString("/")
		buf.WriteString(string(op))
		buf.WriteString("/")
		buf.WriteString(k.KeyString())
		c.keysBuf[ki] = buf.Bytes()
	}
	for _, sop := range c.SpaceOps {
		for _, k := range sop.Add {
			addKey(sop.SpaceId, k, OpAdd)
		}
		for _, k := range sop.Delete {
			addKey(sop.SpaceId, k, OpDelete)
		}
		for _, k := range sop.Load {
			addKey(sop.SpaceId, k, OpLoad)
		}
	}
	return c.keysBuf
}

func (c *Cids) AddKey(key []byte) error {
	spaceId, op, k, err := parseCIDOp(key)
	if err != nil {
		return err
	}
	c.Add(spaceId, op, k)
	return nil
}

func (c *Cids) Len() (l int) {
	for _, so := range c.SpaceOps {
		l += len(so.Load)
		l += len(so.Delete)
		l += len(so.Add)
	}
	return
}

func (c *Cids) Release() {
	c.keysBuf = c.keysBuf[:0]
	for i, sop := range c.SpaceOps {
		c.SpaceOps[i].Add = sop.Add[:0]
		c.SpaceOps[i].Delete = sop.Delete[:0]
		c.SpaceOps[i].Load = sop.Load[:0]
	}
	c.SpaceOps = c.SpaceOps[:0]
	cidsPool.Put(c)
}
