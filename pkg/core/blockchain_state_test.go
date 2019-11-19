package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/stretchr/testify/require"
)

func TestNewBlockChainStateAndCommit(t *testing.T) {
	memCachedStore := storage.NewMemCachedStore(storage.NewMemoryStore())
	bcState := NewBlockChainState(memCachedStore)
	err := bcState.commit()
	require.NoError(t, err)
}

func TestStoreAsBlock(t *testing.T) {
	memCachedStore := storage.NewMemCachedStore(storage.NewMemoryStore())
	bcState := NewBlockChainState(memCachedStore)

	block := newBlock(0, newMinerTX())
	err := bcState.storeAsBlock(block, 0)
	require.NoError(t, err)
}

func TestStoreAsCurrentBlock(t *testing.T) {
	memCachedStore := storage.NewMemCachedStore(storage.NewMemoryStore())
	bcState := NewBlockChainState(memCachedStore)

	block := newBlock(0, newMinerTX())
	err := bcState.storeAsCurrentBlock(block)
	require.NoError(t, err)
}

func TestStoreAsTransaction(t *testing.T) {
	memCachedStore := storage.NewMemCachedStore(storage.NewMemoryStore())
	bcState := NewBlockChainState(memCachedStore)

	tx := &transaction.Transaction{
		Type: transaction.MinerType,
		Data: &transaction.MinerTX{},
	}
	err := bcState.storeAsTransaction(tx, 0)
	require.NoError(t, err)
}
