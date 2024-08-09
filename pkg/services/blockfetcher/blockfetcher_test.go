package blockfetcher

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestProcessBlock(t *testing.T) {
	bc, validators, committee := chain.NewMultiWithCustomConfig(t,
		func(c *config.Blockchain) {
			c.P2PSigExtensions = true
		})
	e := neotest.NewExecutor(t, bc, validators, committee)

	basicchain.Init(t, "../../../", e)
	require.True(t, bc.BlockHeight() > 5)

	w := gio.NewBufBinWriter()
	require.NoError(t, chaindump.Dump(bc, w.BinWriter, 0, bc.BlockHeight()+1))
	require.NoError(t, w.Err)
	buf := w.Bytes()
	bc2, _, _ := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
		c.P2PSigExtensions = true
	})
	cfg := Config{
		ContainerID: "9iVfUg8aDHKjPC4LhQXEkVUM4HDkR7UCXYLs8NQwYfSG",
		Nodes:       []string{"https://st1.t5.fs.neo.org:8080"},
		Timeout:     10,
		DumpDir:     "./",
	}
	serv := New(bc2, storage.NewMemoryStore(), cfg, zaptest.NewLogger(t))
	err := serv.processBlock(buf, bc.BlockHeight()+1)
	require.NoError(t, err)
	require.Equal(t, bc.BlockHeight(), bc2.BlockHeight())
}

func TestService(t *testing.T) {
	bc, _, _ := chain.NewMultiWithCustomConfig(t,
		func(c *config.Blockchain) {
			c.P2PSigExtensions = true
		})
	cfg := Config{
		ContainerID: "9iVfUg8aDHKjPC4LhQXEkVUM4HDkR7UCXYLs8NQwYfSG",
		Nodes:       []string{"https://st1.t5.fs.neo.org:8080"},
		Timeout:     10,
		DumpDir:     "./",
	}
	serv := New(bc, storage.NewMemoryStore(), cfg, zaptest.NewLogger(t))
	require.NotNil(t, serv)
	require.Equal(t, "BlockFetcherService", serv.Name())
	serv.Start()
	fmt.Println(bc.BlockHeight())
}
