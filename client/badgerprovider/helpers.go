package badgerprovider

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
)

var ErrIncorrectKey = errors.New("the key is incorrect")

func Has(db *badger.DB, key []byte) bool {
	return db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		return err
	}) == nil
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

func GetAndCopy(txn *badger.Txn, key []byte) (value []byte, err error) {
	item, err := txn.Get(key)
	if err != nil {
		return
	}
	return item.ValueCopy(value)
}
