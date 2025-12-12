package storage

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/nspcc-dev/bbolt"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Bucket represents bucket used in boltdb to store all the data.
var Bucket = []byte("DB")

// BoltDBStore it is the storage implementation for storing and retrieving
// blockchain data.
type BoltDBStore struct {
	db  *bbolt.DB
	bkt []byte
}

// defaultOpenTimeout is the default timeout for performing flock on a bbolt database.
// bbolt does retries every 50ms during this interval.
const defaultOpenTimeout = 1 * time.Second

// NewBoltDBStore returns a new ready to use BoltDB storage with new bucket
// named [Bucket]. In RO mode this bucket is checked for presence only.
func NewBoltDBStore(cfg dbconfig.BoltDBOptions) (*BoltDBStore, error) {
	cp := *bbolt.DefaultOptions // Do not change bbolt's global variable.
	opts := &cp
	fileMode := os.FileMode(0600) // should be exposed via BoltDBOptions if anything needed
	fileName := cfg.FilePath
	if cfg.ReadOnly {
		opts.ReadOnly = true
	} else {
		if err := io.MakeDirForFile(fileName, "BoltDB"); err != nil {
			return nil, err
		}
	}
	opts.Timeout = defaultOpenTimeout

	db, err := bbolt.Open(fileName, fileMode, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BoltDB instance: %w", err)
	}
	b, err := NewBoltDBStoreFromBucket(db, Bucket)
	if err != nil {
		closeErr := db.Close()
		err = fmt.Errorf("failed to initialize BoltDB instance: %w", err)
		if closeErr != nil {
			err = fmt.Errorf("%w, failed to close BoltDB instance: %w", err, closeErr)
		}
		return nil, err
	}

	return b, nil
}

// NewBoltDBStoreFromBucket creates an instance of [BoltDBStore] from already
// existing and opened DB. Effectively it allows to have multiple [BoltDBStore]
// in the same DB given that user manages buckets properly. If the DB is
// opened in read-only mode the bucket is checked for presence, otherwise
// it's auto-created.
func NewBoltDBStoreFromBucket(db *bbolt.DB, bucket []byte) (*BoltDBStore, error) {
	var err error

	if db.IsReadOnly() {
		err = db.View(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucket)
			if b == nil {
				return errors.New("root bucket does not exist")
			}
			return nil
		})
	} else {
		err = db.Update(func(tx *bbolt.Tx) error {
			_, err = tx.CreateBucketIfNotExists(bucket)
			if err != nil {
				return fmt.Errorf("could not create root bucket: %w", err)
			}
			return nil
		})
	}
	if err != nil {
		return nil, err
	}
	return &BoltDBStore{db: db, bkt: bucket}, nil
}

// Get implements the Store interface.
func (s *BoltDBStore) Get(key []byte) (val []byte, err error) {
	err = s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(s.bkt)
		// Value from Get is only valid for the lifetime of transaction, #1482
		val = bytes.Clone(b.Get(key))
		return nil
	})
	if val == nil {
		err = ErrKeyNotFound
	}
	return
}

// PutChangeSet implements the Store interface.
func (s *BoltDBStore) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	var err error

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(s.bkt)
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
func (s *BoltDBStore) SeekGC(rng SeekRange, keepCont func(k, v []byte) (bool, bool)) error {
	return boltSeek(s.db.Update, s.bkt, rng, func(c *bbolt.Cursor, k, v []byte) (bool, error) {
		keep, cont := keepCont(k, v)
		if !keep {
			if err := c.Delete(); err != nil {
				return false, err
			}
		}
		return cont, nil
	})
}

// Seek implements the Store interface.
func (s *BoltDBStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	err := boltSeek(s.db.View, s.bkt, rng, func(_ *bbolt.Cursor, k, v []byte) (bool, error) {
		return f(k, v), nil
	})
	if err != nil {
		panic(err)
	}
}

func boltSeek(txopener func(func(*bbolt.Tx) error) error, bucket []byte, rng SeekRange, f func(c *bbolt.Cursor, k, v []byte) (bool, error)) error {
	rang := seekRangeToPrefixes(rng)
	return txopener(func(tx *bbolt.Tx) error {
		var (
			k, v []byte
			next func() ([]byte, []byte)
		)

		c := tx.Bucket(bucket).Cursor()

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

// Close releases all db resources.
func (s *BoltDBStore) Close() error {
	return s.db.Close()
}
