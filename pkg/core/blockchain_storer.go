package core

import (
	"log"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// BlockchainStorer is anything that can persist and retrieve the blockchain.
type BlockchainStorer interface {
	HasBlock(util.Uint256) bool
	GetBlockByHeight(uint32) (*Block, error)
	GetBlockByHash(util.Uint256) (*Block, error)
	Put(*Header) error
}

// MemoryStore is an in memory implementation of a BlockChainStorer.
type MemoryStore struct {
	mtx    sync.RWMutex
	blocks map[util.Uint256]*Header
}

// NewMemoryStore returns a pointer to a MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		blocks: map[util.Uint256]*Header{},
	}
}

// HasBlock implements the BlockchainStorer interface.
func (s *MemoryStore) HasBlock(hash util.Uint256) bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	_, ok := s.blocks[hash]
	return ok
}

// GetBlockByHash returns a block by its hash.
func (s *MemoryStore) GetBlockByHash(hash util.Uint256) (*Block, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return nil, nil
}

// GetBlockByHeight returns a block by its height.
func (s *MemoryStore) GetBlockByHeight(i uint32) (*Block, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return nil, nil
}

// Put persist a BlockHead in memory
func (s *MemoryStore) Put(header *Header) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	hash, err := header.Hash()
	if err != nil {
		s.blocks[hash] = header
	}

	log.Printf("persisted block %s\n", hash)

	return err
}
