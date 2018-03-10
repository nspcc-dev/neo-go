package core

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// LevelDBStore is the official storage implementation for storing and retreiving
// blockchain data.
type LevelDBStore struct {
	db   *leveldb.DB
	path string
}

// NewLevelDBStore return a new LevelDBStore object that will
// initialize the database found at the given path.
func NewLevelDBStore(path string, opts *opt.Options) (*LevelDBStore, error) {
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		return nil, err
	}
	return &LevelDBStore{
		path: path,
		db:   db,
	}, nil
}

// Write implements the Store interface.
func (s *LevelDBStore) write(key, value []byte) error {
	return s.db.Put(key, value, nil)
}

// WriteBatch implements the Store interface.
func (s *LevelDBStore) writeBatch(batch Batch) error {
	b := new(leveldb.Batch)
	for k, v := range batch {
		b.Put(*k, v)
	}
	return s.db.Write(b, nil)
}
