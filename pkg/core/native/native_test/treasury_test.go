package native_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
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

	// Invoke at Faun should not fail (but fee is calculated manually since Treasury is not yet initialized during test invocation).
	tx := c.NewUnsignedTx(t, c.Hash, "verify")
	c.SignTx(t, tx, 1_0000_0000, c.Signers...)
	c.AddNewBlock(t, tx)
	c.CheckHalt(t, tx.Hash(), stackitem.Make(true))

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

func TestTreasury_OnNEPPayment_NullSender(t *testing.T) {
	const huyaoHeight = 10
	c := newCustomTreasuryClient(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFFaun.String():   0,
			config.HFGorgon.String(): 0,
			config.HFHuyao.String():  huyaoHeight,
		}
	})
	sender := c.Signers[0].ScriptHash()

	// Before Huyao null sender is rejected, regular sender is accepted.
	c.InvokeFail(t, "invalid conversion", "onNEP17Payment", nil, 1, nil)
	c.InvokeFail(t, "invalid conversion", "onNEP11Payment", nil, 1, []byte{1}, nil)
	c.Invoke(t, stackitem.Null{}, "onNEP17Payment", sender, 1, nil)
	c.Invoke(t, stackitem.Null{}, "onNEP11Payment", sender, 1, []byte{1}, nil)

	// GAS mint to Treasury on NEO balance change fails before Huyao.
	neoInvoker := c.CommitteeInvoker(nativehashes.NeoToken)
	neoInvoker.Invoke(t, true, "transfer", sender, nativehashes.Treasury, 1, nil)
	c.AddNewBlock(t)
	neoInvoker.InvokeFail(t, "invalid conversion", "transfer", sender, nativehashes.Treasury, 1, nil)

	for c.Chain.BlockHeight() < huyaoHeight {
		c.AddNewBlock(t)
	}

	// After Huyao both null and regular senders are accepted.
	c.Invoke(t, stackitem.Null{}, "onNEP17Payment", nil, 1, nil)
	c.Invoke(t, stackitem.Null{}, "onNEP11Payment", nil, 1, []byte{1}, nil)
	c.Invoke(t, stackitem.Null{}, "onNEP17Payment", sender, 1, nil)
	c.Invoke(t, stackitem.Null{}, "onNEP11Payment", sender, 1, []byte{1}, nil)
	c.InvokeFail(t, "expected byte size of 20", "onNEP17Payment", 1, 1, nil)

	neoInvoker.Invoke(t, true, "transfer", sender, nativehashes.Treasury, 1, nil)
	require.Positive(t, c.Chain.GetUtilityTokenBalance(nativehashes.Treasury, util.Uint160{}).Sign())
}
