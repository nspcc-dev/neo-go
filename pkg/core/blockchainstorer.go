package core

import (
	"sync"

	"github.com/anthdm/neo-go/pkg/util"
)

// BlockchainStorer is anything that can persist and retrieve the blockchain.
type BlockchainStorer interface {
	HasBlock(util.Uint256) bool
	GetBlockByHeight(uint32) (*Block, error)
	GetBlockByHash(util.Uint256) (*Block, error)
}

// MemoryStore is an in memory implementation of a BlockChainStorer.
type MemoryStore struct {
	mtx    *sync.RWMutex
	blocks map[util.Uint256]*Block
}

// NewMemoryStore returns a pointer to a MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		mtx:    &sync.RWMutex{},
		blocks: map[util.Uint256]*Block{},
	}
}

// HasBlock implements the BlockchainStorer interface.
func (s *MemoryStore) HasBlock(hash util.Uint256) bool {
	_, ok := s.blocks[hash]
	return ok
}

// GetBlockByHash returns a block by its hash.
func (s *MemoryStore) GetBlockByHash(hash util.Uint256) (*Block, error) {
	return nil, nil
}

// GetBlockByHeight returns a block by its height.
func (s *MemoryStore) GetBlockByHeight(i uint32) (*Block, error) {
	return nil, nil
}
