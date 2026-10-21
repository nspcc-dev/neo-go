package storage

import (
	"errors"
	"fmt"
	"sync"

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

	viewLock sync.RWMutex
	view     *leveldb.Snapshot
}

// NewLevelDBStore returns a new LevelDBStore object that will
// initialize the database found at the given path.
func NewLevelDBStore(cfg dbconfig.LevelDBOptions) (*LevelDBStore, error) {
	var opts = new(opt.Options) // should be exposed via LevelDBOptions if anything needed
	if cfg.ReadOnly {
		opts.ReadOnly = true
		opts.ErrorIfMissing = true
	}
	opts.Filter = filter.NewBloomFilter(10)
	db, err := leveldb.OpenFile(cfg.DataDirectoryPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB instance: %w", err)
	}

	s := &LevelDBStore{
		path: cfg.DataDirectoryPath,
		db:   db,
	}
	s.view, err = s.beginView()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *LevelDBStore) beginView() (*leveldb.Snapshot, error) {
	sh, err := s.db.GetSnapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}
	return sh, nil
}

// GetWithView implements the Store interface.
func (s *LevelDBStore) GetWithView(key []byte) ([]byte, error) {
	s.viewLock.RLock()
	defer s.viewLock.RUnlock()
	value, err := s.view.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		err = ErrKeyNotFound
	}
	return value, err
}

// PutChangeSet implements the Store interface.
func (s *LevelDBStore) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	s.viewLock.Lock()
	s.view.Release()
	defer func() {
		s.view, _ = s.beginView() // TODO: err
		s.viewLock.Unlock()
	}()
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

// SeekWithView implements the Store interface.
func (s *LevelDBStore) SeekWithView(rng SeekRange, f func(k []byte, v []byte) bool) {
	s.viewLock.RLock()
	defer s.viewLock.RUnlock()
	iter := s.view.NewIterator(seekRangeToPrefixes(rng), nil)
	s.seek(iter, rng.Backwards, f)
}

// SeekGC implements the Store interface.
func (s *LevelDBStore) SeekGC(rng SeekRange, keepCont func(k, v []byte) (bool, bool)) error {
	s.viewLock.Lock()
	s.view.Release()
	defer func() {
		s.view, _ = s.beginView() // TODO: err
		s.viewLock.Unlock()
	}()
	tx, err := s.db.OpenTransaction()
	if err != nil {
		return err
	}
	iter := tx.NewIterator(seekRangeToPrefixes(rng), nil)
	s.seek(iter, rng.Backwards, func(k, v []byte) bool {
		keep, cont := keepCont(k, v)
		if !keep {
			err = tx.Delete(k, nil)
			if err != nil {
				return false
			}
		}
		return cont
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
	s.viewLock.Lock()
	s.view.Release()
	s.viewLock.Unlock() // TODO: unlock/db.Close order ?
	return s.db.Close()
}
