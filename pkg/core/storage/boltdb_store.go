package storage

import (
	"bytes"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/syndtr/goleveldb/leveldb/util"
	"go.etcd.io/bbolt"
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
	if err := io.MakeDirForFile(fileName, "BoltDB"); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(fileName, fileMode, opts)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists(Bucket)
		if err != nil {
			return fmt.Errorf("could not create root bucket: %w", err)
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
		// Value from Get is only valid for the lifetime of transaction, #1482
		if val != nil {
			val = slice.Copy(val)
		}
		return nil
	})
	if val == nil {
		err = ErrKeyNotFound
	}
	return
}

// Delete implements the Store interface.
func (s *BoltDBStore) Delete(key []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(Bucket)
		return b.Delete(key)
	})
}

// PutBatch implements the Store interface.
func (s *BoltDBStore) PutBatch(batch Batch) error {
	memBatch := batch.(*MemoryBatch)
	return s.PutChangeSet(memBatch.mem)
}

// PutChangeSet implements the Store interface.
func (s *BoltDBStore) PutChangeSet(puts map[string][]byte) error {
	var err error

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(Bucket)
		for k, v := range puts {
			if v != nil {
				err = b.Put([]byte(k), v)
			} else {
				err = b.Delete([]byte(k))
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Seek implements the Store interface.
func (s *BoltDBStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	start := make([]byte, len(rng.Prefix)+len(rng.Start))
	copy(start, rng.Prefix)
	copy(start[len(rng.Prefix):], rng.Start)
	if rng.Backwards {
		s.seekBackwards(rng.Prefix, start, f)
	} else {
		s.seek(rng.Prefix, start, f)
	}
}

func (s *BoltDBStore) seek(key []byte, start []byte, f func(k, v []byte) bool) {
	prefix := util.BytesPrefix(key)
	prefix.Start = start
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(Bucket).Cursor()
		for k, v := c.Seek(prefix.Start); k != nil && (len(prefix.Limit) == 0 || bytes.Compare(k, prefix.Limit) <= 0); k, v = c.Next() {
			if !f(k, v) {
				break
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func (s *BoltDBStore) seekBackwards(key []byte, start []byte, f func(k, v []byte) bool) {
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(Bucket).Cursor()
		// Move cursor to the first kv pair which is followed by the pair matching the specified prefix.
		if len(start) == 0 {
			lastKey, _ := c.Last()
			start = lastKey
		}
		rng := util.BytesPrefix(start) // in fact, we only need limit based on start slice to iterate backwards starting from this limit
		c.Seek(rng.Limit)
		for k, v := c.Prev(); k != nil && bytes.HasPrefix(k, key); k, v = c.Prev() {
			if !f(k, v) {
				break
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
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
