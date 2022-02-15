package storage

import (
	"bytes"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
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
	return s.PutChangeSet(memBatch.mem, memBatch.stor)
}

// PutChangeSet implements the Store interface.
func (s *BoltDBStore) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	var err error

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(Bucket)
		for _, m := range []map[string][]byte{puts, stores} {
			for k, v := range m {
				if v != nil {
					err = b.Put([]byte(k), v)
				} else {
					err = b.Delete([]byte(k))
				}
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// SeekGC implements the Store interface.
func (s *BoltDBStore) SeekGC(rng SeekRange, keep func(k, v []byte) bool) error {
	return boltSeek(s.db.Update, rng, func(c *bbolt.Cursor, k, v []byte) (bool, error) {
		if !keep(k, v) {
			if err := c.Delete(); err != nil {
				return false, err
			}
		}
		return true, nil
	})
}

// Seek implements the Store interface.
func (s *BoltDBStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	err := boltSeek(s.db.View, rng, func(_ *bbolt.Cursor, k, v []byte) (bool, error) {
		return f(k, v), nil
	})
	if err != nil {
		panic(err)
	}
}

func boltSeek(txopener func(func(*bbolt.Tx) error) error, rng SeekRange, f func(c *bbolt.Cursor, k, v []byte) (bool, error)) error {
	rang := seekRangeToPrefixes(rng)
	return txopener(func(tx *bbolt.Tx) error {
		var (
			k, v []byte
			next func() ([]byte, []byte)
		)

		c := tx.Bucket(Bucket).Cursor()

		if !rng.Backwards {
			k, v = c.Seek(rang.Start)
			next = c.Next
		} else {
			if len(rang.Limit) == 0 {
				lastKey, _ := c.Last()
				k, v = c.Seek(lastKey)
			} else {
				c.Seek(rang.Limit)
				k, v = c.Prev()
			}
			next = c.Prev
		}

		for ; k != nil && bytes.HasPrefix(k, rng.Prefix) && (len(rang.Limit) == 0 || bytes.Compare(k, rang.Limit) <= 0); k, v = next() {
			cont, err := f(c, k, v)
			if err != nil {
				return err
			}
			if !cont {
				break
			}
		}
		return nil
	})
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
