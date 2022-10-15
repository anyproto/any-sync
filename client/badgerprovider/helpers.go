package badgerprovider

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
)

var ErrIncorrectKey = errors.New("the key is incorrect")

func Has(db *badger.DB, key []byte) (has bool, err error) {
	err = db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		return err
	})
	if err != nil {
		return
	}
	has = true
	return
}

func Put(db *badger.DB, key, value []byte) (err error) {
	return db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func Get(db *badger.DB, key []byte) (value []byte, err error) {
	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(value)
		if err != nil {
			return err
		}
		return err
	})
	return
}

func GetKeySuffix(txn *badger.Txn, keyPrefix []byte) (suffix []byte, err error) {
	iter := txn.NewIterator(badger.IteratorOptions{Prefix: keyPrefix})
	iter.Next()
	if !iter.Valid() {
		err = badger.ErrKeyNotFound
		return
	}

	suffix = iter.Item().Key()
	if len(suffix) <= len(keyPrefix)+1 {
		err = ErrIncorrectKey
		return
	}
	suffix = suffix[len(keyPrefix)+1:]
	return
}

func GetKeySuffixAndValue(txn *badger.Txn, keyPrefix []byte) (suffix []byte, value []byte, err error) {
	iter := txn.NewIterator(badger.IteratorOptions{Prefix: keyPrefix})
	iter.Next()
	if !iter.Valid() {
		err = badger.ErrKeyNotFound
		return
	}

	suffix = iter.Item().Key()
	if len(suffix) <= len(keyPrefix)+1 {
		err = ErrIncorrectKey
		return
	}
	suffix = suffix[len(keyPrefix)+1:]
	value, err = iter.Item().ValueCopy(value)
	return
}
