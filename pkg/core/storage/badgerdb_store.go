package storage

import (
	"os"

	"github.com/dgraph-io/badger/v2"
)

// BadgerDBOptions configuration for BadgerDB.
type BadgerDBOptions struct {
	Dir string `yaml:"BadgerDir"`
}

// BadgerDBStore is the official storage implementation for storing and retrieving
// blockchain data.
type BadgerDBStore struct {
	db *badger.DB
}

// BadgerDBBatch is a wrapper around badger.WriteBatch, compatible with Batch interface.
type BadgerDBBatch struct {
	batch *badger.WriteBatch
}

// Delete implements the Batch interface.
func (b *BadgerDBBatch) Delete(key []byte) {
	err := b.batch.Delete(key)
	if err != nil {
		panic(err)
	}
}

// Put implements the Batch interface.
func (b *BadgerDBBatch) Put(key, value []byte) {
	keycopy := make([]byte, len(key))
	copy(keycopy, key)
	valuecopy := make([]byte, len(value))
	copy(valuecopy, value)
	err := b.batch.Set(keycopy, valuecopy)
	if err != nil {
		panic(err)
	}
}

// NewBadgerDBStore returns a new BadgerDBStore object that will
// initialize the database found at the given path.
func NewBadgerDBStore(cfg BadgerDBOptions) (*BadgerDBStore, error) {
	// BadgerDB isn't able to make nested directories
	err := os.MkdirAll(cfg.Dir, os.ModePerm)
	if err != nil {
		panic(err)
	}
	opts := badger.DefaultOptions(cfg.Dir) // should be exposed via BadgerDBOptions if anything needed

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &BadgerDBStore{
		db: db,
	}, nil
}

// Batch implements the Batch interface and returns a badgerdb
// compatible Batch.
func (b *BadgerDBStore) Batch() Batch {
	return &BadgerDBBatch{b.db.NewWriteBatch()}
}

// Delete implements the Store interface.
func (b *BadgerDBStore) Delete(key []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// Get implements the Store interface.
func (b *BadgerDBStore) Get(key []byte) ([]byte, error) {
	var val []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return ErrKeyNotFound
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	return val, err
}

// Put implements the Store interface.
func (b *BadgerDBStore) Put(key, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		err := txn.Set(key, value)
		return err
	})
}

// PutBatch implements the Store interface.
func (b *BadgerDBStore) PutBatch(batch Batch) error {
	defer batch.(*BadgerDBBatch).batch.Cancel()
	return batch.(*BadgerDBBatch).batch.Flush()
}

// Seek implements the Store interface.
func (b *BadgerDBStore) Seek(key []byte, f func(k, v []byte)) {
	err := b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   100,
			Reverse:        false,
			AllVersions:    false,
			Prefix:         key,
			InternalAccess: false,
		})
		defer it.Close()
		for it.Seek(key); it.ValidForPrefix(key); it.Next() {
			item := it.Item()
			k := item.Key()
			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			f(k, v)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// Close releases all db resources.
func (b *BadgerDBStore) Close() error {
	return b.db.Close()
}
