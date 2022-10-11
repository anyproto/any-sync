package db

import (
	"fmt"
	"github.com/dgraph-io/badger/v3"
)

type badgerTree struct {
	id      string
	spaceId string
	db      *badger.DB
}

func (b *badgerTree) Id() string {
	return b.id
}

func (b *badgerTree) UpdateHead(head string) (err error) {
	key := fmt.Sprintf("space/%s/tree/%s/heads", b.spaceId, b.id)
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(head))
	})
}

func (b *badgerTree) AddChange(key string, value []byte) (err error) {
	badgerKey := fmt.Sprintf("space/%s/tree/%s/change/%s", b.spaceId, b.id, key)
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(badgerKey), value)
	})
}

type badgerSpace struct {
	id string
	db *badger.DB
}

func (b *badgerSpace) Id() string {
	return b.id
}

func (b *badgerSpace) CreateTree(id string) (Tree, error) {
	key := fmt.Sprintf("space/%s/tree/%s", b.id, id)
	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte("exists"))
	})
	if err != nil {
		return nil, err
	}
	return &badgerTree{
		id:      id,
		spaceId: b.id,
		db:      b.db,
	}, nil
}

func (b *badgerSpace) GetTree(id string) (Tree, error) {
	//TODO implement me
	panic("implement me")
}

func (b *badgerSpace) Close() error {
	return nil
}

type badgerSpaceCreator struct {
	rootPath string
	db       *badger.DB
}

func (b *badgerSpaceCreator) CreateSpace(id string) (Space, error) {
	key := fmt.Sprintf("space/%s", id)
	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte("exists"))
	})
	if err != nil {
		return nil, err
	}
	return &badgerSpace{
		id: id,
		db: b.db,
	}, nil
}

func (b *badgerSpaceCreator) GetSpace(id string) (Space, error) {
	key := fmt.Sprintf("space/%s", id)
	err := b.db.Update(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &badgerSpace{
		id: id,
		db: b.db,
	}, nil
}

func (b *badgerSpaceCreator) Close() error {
	return b.db.Close()
}

func NewBadgerSpaceCreator() SpaceCreator {
	rootPath := "badger.db.test"
	db, err := badger.Open(badger.DefaultOptions(rootPath))
	if err != nil {
		panic(err)
	}
	return &badgerSpaceCreator{
		rootPath: rootPath,
		db:       db,
	}
}
