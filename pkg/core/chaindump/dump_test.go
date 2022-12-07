package chaindump_test

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/stretchr/testify/require"
)

func TestBlockchain_DumpAndRestore(t *testing.T) {
	t.Run("no state root", func(t *testing.T) {
		testDumpAndRestore(t, func(c *config.Blockchain) {
			c.StateRootInHeader = false
			c.P2PSigExtensions = true
		}, nil)
	})
	t.Run("with state root", func(t *testing.T) {
		testDumpAndRestore(t, func(c *config.Blockchain) {
			c.StateRootInHeader = true
			c.P2PSigExtensions = true
		}, nil)
	})
	t.Run("remove untraceable", func(t *testing.T) {
		// Dump can only be created if all blocks and transactions are present.
		testDumpAndRestore(t, func(c *config.Blockchain) {
			c.P2PSigExtensions = true
		}, func(c *config.Blockchain) {
			c.MaxTraceableBlocks = 2
			c.Ledger.RemoveUntraceableBlocks = true
			c.P2PSigExtensions = true
		})
	})
}

func testDumpAndRestore(t *testing.T, dumpF, restoreF func(c *config.Blockchain)) {
	if restoreF == nil {
		restoreF = dumpF
	}

	bc, validators, committee := chain.NewMultiWithCustomConfig(t, dumpF)
	e := neotest.NewExecutor(t, bc, validators, committee)

	basicchain.Init(t, "../../../", e)
	require.True(t, bc.BlockHeight() > 5) // ensure that test is valid

	w := io.NewBufBinWriter()
	require.NoError(t, chaindump.Dump(bc, w.BinWriter, 0, bc.BlockHeight()+1))
	require.NoError(t, w.Err)

	buf := w.Bytes()
	t.Run("invalid start", func(t *testing.T) {
		bc2, _, _ := chain.NewMultiWithCustomConfig(t, restoreF)

		r := io.NewBinReaderFromBuf(buf)
		require.Error(t, chaindump.Restore(bc2, r, 2, 1, nil))
	})
	t.Run("good", func(t *testing.T) {
		bc2, _, _ := chain.NewMultiWithCustomConfig(t, dumpF)

		r := io.NewBinReaderFromBuf(buf)
		require.NoError(t, chaindump.Restore(bc2, r, 0, 2, nil))
		require.Equal(t, uint32(1), bc2.BlockHeight())

		r = io.NewBinReaderFromBuf(buf) // new reader because start is relative to dump
		require.NoError(t, chaindump.Restore(bc2, r, 2, 1, nil))
		t.Run("check handler", func(t *testing.T) {
			lastIndex := uint32(0)
			errStopped := errors.New("stopped")
			f := func(b *block.Block) error {
				lastIndex = b.Index
				if b.Index >= bc.BlockHeight()-1 {
					return errStopped
				}
				return nil
			}
			require.NoError(t, chaindump.Restore(bc2, r, 0, 1, f))
			require.Equal(t, bc2.BlockHeight(), lastIndex)

			r = io.NewBinReaderFromBuf(buf)
			err := chaindump.Restore(bc2, r, 4, bc.BlockHeight()-bc2.BlockHeight(), f)
			require.True(t, errors.Is(err, errStopped))
			require.Equal(t, bc.BlockHeight()-1, lastIndex)
		})
	})
}
