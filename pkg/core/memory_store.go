package core

// MemoryStore is an in memory implementation of a BlockChainStorer
// that should only be used for testing.
type MemoryStore struct {
}

// NewMemoryStore returns a pointer to a MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) write(key, value []byte) error {
	return nil
}

func (m *MemoryStore) writeBatch(batch Batch) error {
	for k, v := range batch {
		if err := m.write(*k, v); err != nil {
			return err
		}
	}
	return nil
}
