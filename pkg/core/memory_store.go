package core

// MemoryStore is an in memory implementation of a Store.
// MemoryStore should only be used for testing.
type MemoryStore struct{}

// NewMemoryStore returns a pointer to a MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Get implementes the Store interface.
func (m *MemoryStore) Get(key []byte) ([]byte, error) {
	return nil, errKeyNotFound
}

// Put implementes the Store interface.
func (m *MemoryStore) Put(key, value []byte) error {
	return nil
}

// PutBatch implementes the Store interface.
func (m *MemoryStore) PutBatch(batch Batch) error {
	return nil
}

// Find implementes the Store interface.
func (m *MemoryStore) Find(key []byte, f func(k, v []byte)) {
}
