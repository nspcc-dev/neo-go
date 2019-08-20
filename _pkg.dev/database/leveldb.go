package database

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	ldbutil "github.com/syndtr/goleveldb/leveldb/util"
)

//DbDir is the folder which all database files will be put under
// Structure /DbDir/net
const DbDir = "db/"

// LDB represents a leveldb object
type LDB struct {
	db   *leveldb.DB
	Path string
}

// ErrNotFound means that the value was not found in the db
var ErrNotFound = errors.New("value not found for that key")

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
	//Prefix returns all values that start with key
	Prefix(key []byte) ([][]byte, error)
	// Close closes the underlying db object
	Close() error
}

// New will return a new leveldb instance
func New(path string) (*LDB, error) {
	dbPath := DbDir + path
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(path, nil)
		if err != nil {
			return nil, err
		}
	}

	return &LDB{
		db,
		dbPath,
	}, nil
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
	val, err := l.db.Get(key, nil)
	if err == nil {
		return val, nil
	}
	if err == leveldb.ErrNotFound {
		return val, ErrNotFound
	}
	return val, err

}

// Delete implements the database interface
func (l *LDB) Delete(key []byte) error {
	return l.db.Delete(key, nil)
}

// Close implements the database interface
func (l *LDB) Close() error {
	return l.db.Close()
}

// Prefix implements the database interface
func (l *LDB) Prefix(key []byte) ([][]byte, error) {

	var results [][]byte

	iter := l.db.NewIterator(ldbutil.BytesPrefix(key), nil)
	for iter.Next() {

		value := iter.Value()

		// Copy the data, as we cannot modify it
		// Once the iter has been released
		deref := make([]byte, len(value))

		copy(deref, value)

		// Append result
		results = append(results, deref)

	}
	iter.Release()
	err := iter.Error()
	return results, err
}
