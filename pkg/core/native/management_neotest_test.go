package native_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestManagement_GetNEP17Contracts(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		bc, validators, committee := chain.NewMulti(t)
		e := neotest.NewExecutor(t, bc, validators, committee)

		require.ElementsMatch(t, []util.Uint160{e.NativeHash(t, nativenames.Neo),
			e.NativeHash(t, nativenames.Gas)}, bc.GetNEP17Contracts())
	})

	t.Run("basic chain", func(t *testing.T) {
		bc, validators, committee := chain.NewMultiWithCustomConfig(t, func(c *config.ProtocolConfiguration) {
			c.P2PSigExtensions = true // `basicchain.Init` requires Notary enabled
		})
		e := neotest.NewExecutor(t, bc, validators, committee)
		basicchain.Init(t, "../../../", e)

		require.ElementsMatch(t, []util.Uint160{e.NativeHash(t, nativenames.Neo),
			e.NativeHash(t, nativenames.Gas), e.ContractHash(t, 1)}, bc.GetNEP17Contracts())
	})
}
