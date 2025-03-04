package storage

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// LevelDBStore is the official storage implementation for storing and retrieving
// blockchain data.
type LevelDBStore struct {
	db   *leveldb.DB
	path string
}

// NewLevelDBStore returns a new LevelDBStore object that will
// initialize the database found at the given path.
func NewLevelDBStore(cfg dbconfig.LevelDBOptions) (*LevelDBStore, error) {
	var opts = new(opt.Options)
	if cfg.ReadOnly {
		opts.ReadOnly = true
		opts.ErrorIfMissing = true
	}

	// Set filter
	opts.Filter = filter.NewBloomFilter(10)

	// Apply custom options if set
	if cfg.WriteBufferSize != "" {
		val, err := dbconfig.EvaluateExpression(cfg.WriteBufferSize)
		if err != nil {
			return nil, fmt.Errorf("invalid WriteBufferSize: %w", err)
		}
		if val > 0 {
			opts.WriteBuffer = val
		}
	}

	if cfg.BlockSize != "" {
		val, err := dbconfig.EvaluateExpression(cfg.BlockSize)
		if err != nil {
			return nil, fmt.Errorf("invalid BlockSize: %w", err)
		}
		if val > 0 {
			opts.BlockSize = val
		}
	}

	if cfg.BlockCacheCapacity != "" {
		val, err := dbconfig.EvaluateExpression(cfg.BlockCacheCapacity)
		if err != nil {
			return nil, fmt.Errorf("invalid BlockCacheCapacity: %w", err)
		}
		if val > 0 {
			opts.BlockCacheCapacity = val
		}
	}

	if cfg.CompactionTableSize != "" {
		val, err := dbconfig.EvaluateExpression(cfg.CompactionTableSize)
		if err != nil {
			return nil, fmt.Errorf("invalid CompactionTableSize: %w", err)
		}
		if val > 0 {
			opts.CompactionTableSize = val
		}
	}

	if cfg.CompactionL0Trigger > 0 {
		opts.CompactionL0Trigger = cfg.CompactionL0Trigger
	}

	if cfg.OpenFilesCacheCapacity > 0 {
		opts.OpenFilesCacheCapacity = cfg.OpenFilesCacheCapacity
	}

	db, err := leveldb.OpenFile(cfg.DataDirectoryPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB instance: %w", err)
	}

	return &LevelDBStore{
		path: cfg.DataDirectoryPath,
		db:   db,
	}, nil
}

// Get implements the Store interface.
func (s *LevelDBStore) Get(key []byte) ([]byte, error) {
	value, err := s.db.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		err = ErrKeyNotFound
	}
	return value, err
}

// PutChangeSet implements the Store interface.
func (s *LevelDBStore) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	tx, err := s.db.OpenTransaction()
	if err != nil {
		return err
	}
	for _, m := range []map[string][]byte{puts, stores} {
		for k := range m {
			if m[k] != nil {
				err = tx.Put([]byte(k), m[k], nil)
			} else {
				err = tx.Delete([]byte(k), nil)
			}
			if err != nil {
				tx.Discard()
				return err
			}
		}
	}
	return tx.Commit()
}

// Seek implements the Store interface.
func (s *LevelDBStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	iter := s.db.NewIterator(seekRangeToPrefixes(rng), nil)
	s.seek(iter, rng.Backwards, f)
}

// SeekGC implements the Store interface.
func (s *LevelDBStore) SeekGC(rng SeekRange, keep func(k, v []byte) bool) error {
	tx, err := s.db.OpenTransaction()
	if err != nil {
		return err
	}
	iter := tx.NewIterator(seekRangeToPrefixes(rng), nil)
	s.seek(iter, rng.Backwards, func(k, v []byte) bool {
		if !keep(k, v) {
			err = tx.Delete(k, nil)
			if err != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *LevelDBStore) seek(iter iterator.Iterator, backwards bool, f func(k, v []byte) bool) {
	var (
		next func() bool
		ok   bool
	)

	if !backwards {
		ok = iter.Next()
		next = iter.Next
	} else {
		ok = iter.Last()
		next = iter.Prev
	}

	for ; ok; ok = next() {
		if !f(iter.Key(), iter.Value()) {
			break
		}
	}
	iter.Release()
}

// Close implements the Store interface.
func (s *LevelDBStore) Close() error {
	return s.db.Close()
}
