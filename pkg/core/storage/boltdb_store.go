package storage

import (
	"bytes"
	"fmt"
	"os"
	"path"

	"github.com/etcd-io/bbolt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// BoltDBOptions configuration for boltdb.
type BoltDBOptions struct {
	FilePath string `yaml:"FilePath"`
}

// Bucket represents bucket used in boltdb to store all the data.
var Bucket = []byte("DB")

// BoltDBStore it is the storage implementation for storing and retrieving
// blockchain data.
type BoltDBStore struct {
	db *bbolt.DB
}

// NewBoltDBStore returns a new ready to use BoltDB storage with created bucket.
func NewBoltDBStore(cfg BoltDBOptions) (*BoltDBStore, error) {
	var opts *bbolt.Options       // should be exposed via BoltDBOptions if anything needed
	fileMode := os.FileMode(0600) // should be exposed via BoltDBOptions if anything needed
	fileName := cfg.FilePath
	dir := path.Dir(fileName)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("could not create dir for BoltDB: %v", err)
	}
	db, err := bbolt.Open(fileName, fileMode, opts)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists(Bucket)
		if err != nil {
			return fmt.Errorf("could not create root bucket: %v", err)
		}
		return nil
	})

	return &BoltDBStore{db: db}, nil
}

// Put implements the Store interface.
func (s *BoltDBStore) Put(key, value []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(Bucket)
		err := b.Put(key, value)
		return err
	})
}

// Get implements the Store interface.
func (s *BoltDBStore) Get(key []byte) (val []byte, err error) {
	err = s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(Bucket)
		val = b.Get(key)
		return nil
	})
	if val == nil {
		err = ErrKeyNotFound
	}
	return
}

// PutBatch implements the Store interface.
func (s *BoltDBStore) PutBatch(batch Batch) error {
	return s.db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket(Bucket)
		for k, v := range batch.(*MemoryBatch).m {
			err := b.Put([]byte(k), v)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Seek implements the Store interface.
func (s *BoltDBStore) Seek(key []byte, f func(k, v []byte)) {
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(Bucket).Cursor()
		prefix := util.BytesPrefix(key)
		for k, v := c.Seek(prefix.Start); k != nil && bytes.Compare(k, prefix.Limit) <= 0; k, v = c.Next() {
			f(k, v)
		}
		return nil
	})
	if err != nil {
		fmt.Println("error while executing seek in boltDB")
	}
}

// Batch implements the Batch interface and returns a boltdb
// compatible Batch.
func (s *BoltDBStore) Batch() Batch {
	return newMemoryBatch()
}

// Close releases all db resources.
func (s *BoltDBStore) Close() error {
	return s.db.Close()
}
