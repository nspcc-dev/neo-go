package core_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/stretchr/testify/require"
)

const (
	// basicChainPrefix is a prefix used to store Basic chain .acc file for tests.
	// It is also used to retrieve smart contracts that should be deployed to
	// Basic chain.
	basicChainPrefix = "../services/rpcsrv/testdata/"
	// bcPersistInterval is the time period Blockchain persists changes to the
	// underlying storage.
	bcPersistInterval = time.Second
)

var (
	pathToInternalContracts = filepath.Join("..", "..", "internal", "contracts")
)

// TestCreateBasicChain generates "pkg/services/rpcsrv/testdata/testblocks.acc" file which
// contains data for RPC unit tests. It also is a nice integration test.
// To generate new "pkg/services/rpcsrv/testdata/testblocks.acc", follow the steps:
//  1. Set saveChain down below to true
//  2. Run tests with `$ make test`
//  3. Update relevant constants and variables used by RPC server unit tests
//     in "pkg/services/rpcsrv/server_test.go".
func TestCreateBasicChain(t *testing.T) {
	const saveChain = false

	bc, validators, committee := chain.NewMultiWithCustomConfig(t, func(cfg *config.Blockchain) {
		cfg.P2PSigExtensions = true
	})
	e := neotest.NewExecutor(t, bc, validators, committee)

	basicchain.Init(t, "../../", e)

	if saveChain {
		outStream, err := os.Create(basicChainPrefix + "testblocks.acc")
		require.NoError(t, err)
		t.Cleanup(func() {
			outStream.Close()
		})

		writer := io.NewBinWriterFromIO(outStream)
		writer.WriteU32LE(bc.BlockHeight())
		err = chaindump.Dump(bc, writer, 1, bc.BlockHeight())
		require.NoError(t, err)
	}

	require.False(t, saveChain)
}
