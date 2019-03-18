package database

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

// LDB represents a leveldb object
type LDB struct {
	db   *leveldb.DB
	path string
}

// Database contains all methods needed for an object to be a database
type Database interface {
	// Has checks whether the key is in the database
	Has(key []byte) (bool, error)
	// Put adds the key value pair into the pair
	Put(key []byte, value []byte) error
	// Get returns the value for the given key
	Get(key []byte) ([]byte, error)
	// Delete deletes the given value for the key from the database
	Delete(key []byte) error
	// Close closes the underlying db object
	Close() error
}

// New will return a new leveldb instance
func New(path string) *LDB {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil
	}

	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}

	return &LDB{
		db,
		path,
	}
}

// Has implements the database interface
func (l *LDB) Has(key []byte) (bool, error) {
	return l.db.Has(key, nil)
}

// Put implements the database interface
func (l *LDB) Put(key []byte, value []byte) error {
	return l.db.Put(key, value, nil)
}

// Get implements the database interface
func (l *LDB) Get(key []byte) ([]byte, error) {
	return l.db.Get(key, nil)
}

// Delete implements the database interface
func (l *LDB) Delete(key []byte) error {
	return l.db.Delete(key, nil)
}

// Close implements the database interface
func (l *LDB) Close() error {
	return l.db.Close()
}
