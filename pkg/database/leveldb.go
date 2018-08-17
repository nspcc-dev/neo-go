package database

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

type LDB struct {
	db   *leveldb.DB
	path string
}

func New(path string) *LDB {
	db, err := leveldb.OpenFile(path, nil)

	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}

	if err != nil {
		return nil
	}

	return &LDB{
		db,
		path,
	}
}

func (l *LDB) Has(key []byte) (bool, error) {
	return l.db.Has(key, nil)
}

func (l *LDB) Put(key []byte, value []byte) error {
	return l.db.Put(key, value, nil)
}
func (l *LDB) Get(key []byte) ([]byte, error) {
	return l.db.Get(key, nil)
}
func (l *LDB) Delete(key []byte) error {
	return l.db.Delete(key, nil)
}
func (l *LDB) Close() error {
	return l.db.Close()
}
