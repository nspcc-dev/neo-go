package native_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
)

func newTreasuryClient(t *testing.T) *neotest.ContractInvoker {
	return newCustomTreasuryClient(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFFaun.String(): 0,
		}
	})
}

// newCustomTreasuryClient returns native Treasury invoker backed with chain with
// specified custom configuration.
func newCustomTreasuryClient(t *testing.T, f func(cfg *config.Blockchain)) *neotest.ContractInvoker {
	bc, acc := chain.NewSingleWithCustomConfig(t, f)
	e := neotest.NewExecutor(t, bc, acc, acc)

	return e.CommitteeInvoker(nativehashes.Treasury)
}

func TestTreasury_Activation(t *testing.T) {
	c := newCustomTreasuryClient(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFFaun.String(): 2,
		}
	})

	// Invoke before Faun should fail.
	c.InvokeFail(t, fmt.Sprintf("called contract %s not found: key not found", nativehashes.Treasury.StringLE()), "verify")

	// Invoke at Faun should fail.
	c.InvokeWithFeeFail(t, "System.Contract.CallNative failed: native contract Treasury is active after hardfork Faun", 10000_0000, "verify")

	// Invoke after Faun should succeed.
	c.Invoke(t, true, "verify")
}

func TestTreasury_Verify(t *testing.T) {
	c := newTreasuryClient(t)

	// Committee verification passes.
	c.Invoke(t, true, "verify")

	// Side invoker verification fails.
	c.WithSigners(c.NewAccount(t)).Invoke(t, false, "verify")
}

func TestTreasury_OnNEP17Payment(t *testing.T) {
	g := newCustomNativeClient(t, nativenames.Gas, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFFaun.String(): 0,
		}
	})

	g.Invoke(t, true, "transfer", g.Signers[0].ScriptHash(), nativehashes.Treasury, 1, nil)
}
