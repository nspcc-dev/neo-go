package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	*storage.MemoryStore
}

func (memoryStore) Close() error { return nil }

func TestManagement_GetNEP17Contracts(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		chain := newTestChain(t)
		require.ElementsMatch(t, []util.Uint160{chain.contracts.NEO.Hash, chain.contracts.GAS.Hash}, chain.contracts.Management.GetNEP17Contracts())
	})

	t.Run("test chain", func(t *testing.T) {
		chain := newTestChain(t)
		initBasicChain(t, chain)
		rublesHash, err := chain.GetContractScriptHash(1)
		require.NoError(t, err)
		require.ElementsMatch(t, []util.Uint160{chain.contracts.NEO.Hash, chain.contracts.GAS.Hash, rublesHash}, chain.contracts.Management.GetNEP17Contracts())
	})
}
