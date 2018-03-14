package core

import "github.com/syndtr/goleveldb/leveldb"

// MemoryStore is an in memory implementation of a BlockChainStorer
// that should only be used for testing.
type MemoryStore struct{}

// NewMemoryStore returns a pointer to a MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// get implementes the BlockchainStorer interface.
func (m *MemoryStore) get(key []byte) ([]byte, error) {
	return nil, nil
}

// write implementes the BlockchainStorer interface.
func (m *MemoryStore) write(key, value []byte) error {
	return nil
}

// writeBatch implementes the BlockchainStorer interface.
func (m *MemoryStore) writeBatch(batch *leveldb.Batch) error {
	return nil
}
