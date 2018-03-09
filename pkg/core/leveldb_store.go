package core

import (
	"github.com/syndtr/goleveldb/leveldb"
)

// LevelDBStore is the official storage implementation for storing and retreiving
// the blockchain.
type LevelDBStore struct {
	db *leveldb.DB
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
