package storage

import (
	"context"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// LevelDBStore is the official storage implementation for storing and retrieving
// blockchain data.
type LevelDBStore struct {
	db   *leveldb.DB
	path string
}

// NewLevelDBStore return a new LevelDBStore object that will
// initialize the database found at the given path.
func NewLevelDBStore(ctx context.Context, path string, opts *opt.Options) (*LevelDBStore, error) {
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		return nil, err
	}

	// graceful shutdown
	go func() {
		<-ctx.Done()
		db.Close()
	}()

	return &LevelDBStore{
		path: path,
		db:   db,
	}, nil
}

// Put implements the Store interface.
func (s *LevelDBStore) Put(key, value []byte) error {
	return s.db.Put(key, value, nil)
}

// Get implements the Store interface.
func (s *LevelDBStore) Get(key []byte) ([]byte, error) {
	return s.db.Get(key, nil)
}

// PutBatch implements the Store interface.
func (s *LevelDBStore) PutBatch(batch Batch) error {
	lvldbBatch := batch.(*leveldb.Batch)
	return s.db.Write(lvldbBatch, nil)
}

// Seek implements the Store interface.
func (s *LevelDBStore) Seek(key []byte, f func(k, v []byte)) {
	iter := s.db.NewIterator(util.BytesPrefix(key), nil)
	for iter.Next() {
		f(iter.Key(), iter.Value())
	}
	iter.Release()
}

// Batch implements the Batch interface and returns a leveldb
// compatible Batch.
func (s *LevelDBStore) Batch() Batch {
	return new(leveldb.Batch)
}
