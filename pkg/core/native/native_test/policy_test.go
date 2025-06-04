package native_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newPolicyClient(t *testing.T) *neotest.ContractInvoker {
	return newNativeClient(t, nativenames.Policy)
}

func TestPolicy_FeePerByte(t *testing.T) {
	testGetSet(t, newPolicyClient(t), "FeePerByte", 1000, 0, 100_000_000)
}

func TestPolicy_FeePerByteCache(t *testing.T) {
	testGetSetCache(t, newPolicyClient(t), "FeePerByte", 1000)
}

func TestPolicy_ExecFeeFactor(t *testing.T) {
	testGetSet(t, newPolicyClient(t), "ExecFeeFactor", interop.DefaultBaseExecFee, 1, 1000)
}

func TestPolicy_ExecFeeFactorCache(t *testing.T) {
	testGetSetCache(t, newPolicyClient(t), "ExecFeeFactor", interop.DefaultBaseExecFee)
}

func TestPolicy_StoragePrice(t *testing.T) {
	testGetSet(t, newPolicyClient(t), "StoragePrice", native.DefaultStoragePrice, 1, 10000000)
}

func TestPolicy_StoragePriceCache(t *testing.T) {
	testGetSetCache(t, newPolicyClient(t), "StoragePrice", native.DefaultStoragePrice)
}

func TestPolicy_MaxVUBIncrement(t *testing.T) {
	c := newCustomNativeClient(t, nativenames.Policy, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFEchidna.String(): 3,
		}
	})
	committeeInvoker := c.WithSigners(c.Committee)
	name := "MaxValidUntilBlockIncrement"

	t.Run("set, before Echidna", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "method not found: setMaxValidUntilBlockIncrement/1", "set"+name, 123)
	})
	t.Run("get, before Echidna", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "method not found: getMaxValidUntilBlockIncrement/0", "get"+name)
	})

	c.AddNewBlock(t) // enable Echidna.
	testGetSet(t, c, name, int64(c.Chain.GetConfig().Genesis.MaxValidUntilBlockIncrement), 1, 86400)

	t.Run("set, higher than MaxTraceableBlocks", func(t *testing.T) {
		mtb := committeeInvoker.Chain.GetMaxTraceableBlocks()
		committeeInvoker.InvokeFail(t, fmt.Sprintf("MaxValidUntilBlockIncrement should be less than MaxTraceableBlocks %d", mtb), "set"+name, mtb+1)
	})
}

func TestPolicy_MaxVUBIncrementCache(t *testing.T) {
	c := newCustomNativeClient(t, nativenames.Policy, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFEchidna.String(): 1,
		}
	})
	c.AddNewBlock(t) // enable Echidna.
	testGetSetCache(t, c, "MaxValidUntilBlockIncrement", int64(c.Chain.GetConfig().Genesis.MaxValidUntilBlockIncrement))
}

func TestPolicy_MillisecondsPerBlock(t *testing.T) {
	c := newCustomNativeClient(t, nativenames.Policy, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFEchidna.String(): 3,
		}
	})
	committeeInvoker := c.WithSigners(c.Committee)
	name := "MillisecondsPerBlock"

	t.Run("set, before Echidna", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "method not found: setMillisecondsPerBlock/1", "set"+name, 123)
	})
	t.Run("get, before Echidna", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "method not found: getMillisecondsPerBlock/0", "get"+name)
	})

	c.AddNewBlock(t) // enable Echidna.
	testGetSet(t, c, name, c.Chain.GetConfig().Genesis.TimePerBlock.Milliseconds(), 1, 30_000)
}

// TestPolicy_InitializeAtEchidna ensures that native Policy storage/cache initialization is
// performed properly at Echidna fork.
func TestPolicy_InitializeAtEchidna(t *testing.T) {
	check := func(t *testing.T, f func(cfg *config.Blockchain)) {
		c := newCustomNativeClient(t, nativenames.Policy, f)
		committeeInvoker := c.WithSigners(c.Committee)
		defaultTimePerBlock := uint32(c.Chain.GetConfig().TimePerBlock.Milliseconds())
		genesisTimePerBlock := uint32(c.Chain.GetConfig().Genesis.TimePerBlock.Milliseconds())

		echidnaH := int(c.Chain.GetConfig().Hardforks[config.HFEchidna.String()])
		// Pre-Echidna blocks.
		for range int(echidnaH) - 1 {
			require.Equal(t, defaultTimePerBlock, c.Chain.GetMillisecondsPerBlock())
			committeeInvoker.InvokeFail(t, "method not found: getMillisecondsPerBlock/0", "getMillisecondsPerBlock")
			require.Equal(t, defaultTimePerBlock, c.Chain.GetMillisecondsPerBlock())
		}

		// Echidna block.
		if echidnaH > 0 {
			require.Equal(t, defaultTimePerBlock, c.Chain.GetMillisecondsPerBlock())
			committeeInvoker.InvokeWithFee(t, genesisTimePerBlock, 1_0000_0000, "getMillisecondsPerBlock") // use custom fee because test invocation will fail since Echidna is not yet enabled.
			require.Equal(t, genesisTimePerBlock, c.Chain.GetMillisecondsPerBlock())
		}

		// A couple of Post-Echidna blocks.
		for range 2 {
			// Negative echidnaH corresponds to disabled Echidna.
			expected := defaultTimePerBlock
			if echidnaH >= 0 {
				expected = genesisTimePerBlock
			}
			require.Equal(t, expected, c.Chain.GetMillisecondsPerBlock())
			if echidnaH >= 0 {
				committeeInvoker.Invoke(t, expected, "getMillisecondsPerBlock")
			} else {
				committeeInvoker.InvokeFail(t, "method not found: getMillisecondsPerBlock/0", "getMillisecondsPerBlock")
			}
			require.Equal(t, expected, c.Chain.GetMillisecondsPerBlock())
		}
	}
	t.Run("empty hardforks section", func(t *testing.T) {
		check(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = nil
			cfg.Genesis.TimePerBlock = 123 * time.Millisecond
		})
	})
	t.Run("all hardforks explicitly enabled from genesis", func(t *testing.T) {
		check(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
				config.HFCockatrice.String():    0,
				config.HFDomovoi.String():       0,
				config.HFEchidna.String():       0,
			}
			cfg.Genesis.TimePerBlock = 123 * time.Millisecond
		})
	})
	t.Run("Echidna enabled from 2", func(t *testing.T) {
		check(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
				config.HFCockatrice.String():    0,
				config.HFDomovoi.String():       0,
				config.HFEchidna.String():       2,
			}
			cfg.Genesis.TimePerBlock = 123 * time.Millisecond
		})
	})
	t.Run("Domovoi and Echidna enabled from 2", func(t *testing.T) {
		check(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
				config.HFCockatrice.String():    0,
				config.HFDomovoi.String():       2,
				config.HFEchidna.String():       2,
			}
			cfg.Genesis.TimePerBlock = 123 * time.Millisecond
		})
	})
	t.Run("Echidna enabled from 4", func(t *testing.T) {
		check(t, func(cfg *config.Blockchain) {
			cfg.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
				config.HFCockatrice.String():    0,
				config.HFDomovoi.String():       2,
				config.HFEchidna.String():       4,
			}
			cfg.Genesis.TimePerBlock = 123 * time.Millisecond
		})
	})
}

func TestPolicy_MillisecondsPerBlockCache(t *testing.T) {
	c := newCustomNativeClient(t, nativenames.Policy, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFEchidna.String(): 1,
		}
	})
	c.AddNewBlock(t) // enable Echidna.
	testGetSetCache(t, c, "MillisecondsPerBlock", c.Chain.GetConfig().Genesis.TimePerBlock.Milliseconds())
}

func TestPolicy_MaxTraceableBlocks(t *testing.T) {
	c := newCustomNativeClient(t, nativenames.Policy, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFEchidna.String(): 3,
		}
	})
	committeeInvoker := c.WithSigners(c.Committee)
	name := "MaxTraceableBlocks"

	t.Run("set, before Echidna", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "method not found: setMaxTraceableBlocks/1", "set"+name, 123)
	})
	t.Run("get, before Echidna", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "method not found: getMaxTraceableBlocks/0", "get"+name)
	})

	c.AddNewBlock(t) // enable Echidna.
	testGetSet(t, c, name, int64(c.Chain.GetConfig().Genesis.MaxTraceableBlocks), int64(c.Chain.GetConfig().MaxValidUntilBlockIncrement)+1, 2102400)

	t.Run("set, increase", func(t *testing.T) {
		mtb := committeeInvoker.Chain.GetMaxTraceableBlocks()
		committeeInvoker.InvokeFail(t, fmt.Sprintf("MaxTraceableBlocks should not be greater than previous value %d", mtb), "set"+name, mtb+1)
	})

	t.Run("set, lower than MaxValidUntilBlockIncrement", func(t *testing.T) {
		mtb := committeeInvoker.Chain.GetMaxTraceableBlocks()
		committeeInvoker.Invoke(t, stackitem.Null{}, "setMaxValidUntilBlockIncrement", mtb-1)
		committeeInvoker.InvokeFail(t, fmt.Sprintf("MaxTraceableBlocks should be larger than MaxValidUntilBlockIncrement %d", mtb-1), "set"+name, mtb-1)
	})
}

func TestPolicy_MaxTraceableBlocksCache(t *testing.T) {
	c := newCustomNativeClient(t, nativenames.Policy, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFEchidna.String(): 1,
		}
	})
	c.AddNewBlock(t) // enable Echidna.
	testGetSetCache(t, c, "MaxTraceableBlocks", int64(c.Chain.GetConfig().Genesis.MaxTraceableBlocks))
}

func TestPolicy_AttributeFee(t *testing.T) {
	c := newPolicyClient(t)
	getName := "getAttributeFee"
	setName := "setAttributeFee"

	randomInvoker := c.WithSigners(c.NewAccount(t))
	committeeInvoker := c.WithSigners(c.Committee)

	t.Run("set, not signed by committee", func(t *testing.T) {
		randomInvoker.InvokeFail(t, "invalid committee signature", setName, byte(transaction.ConflictsT), 123)
	})
	t.Run("get, unknown attribute", func(t *testing.T) {
		randomInvoker.InvokeFail(t, "invalid attribute type: 84", getName, byte(0x54))
	})
	t.Run("get, default value", func(t *testing.T) {
		randomInvoker.Invoke(t, 0, getName, byte(transaction.ConflictsT))
	})
	t.Run("set, too large value", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "out of range", setName, byte(transaction.ConflictsT), 10_0000_0001)
	})
	t.Run("set, unknown attribute", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "invalid attribute type: 84", setName, 0x54, 5)
	})
	t.Run("set, success", func(t *testing.T) {
		// Set and get in the same block.
		txSet := committeeInvoker.PrepareInvoke(t, setName, byte(transaction.ConflictsT), 1)
		txGet := randomInvoker.PrepareInvoke(t, getName, byte(transaction.ConflictsT))
		c.AddNewBlock(t, txSet, txGet)
		c.CheckHalt(t, txSet.Hash(), stackitem.Null{})
		c.CheckHalt(t, txGet.Hash(), stackitem.Make(1))
		// Get in the next block.
		randomInvoker.Invoke(t, 1, getName, byte(transaction.ConflictsT))
	})
}

func TestPolicy_AttributeFeeCache(t *testing.T) {
	c := newPolicyClient(t)
	getName := "getAttributeFee"
	setName := "setAttributeFee"

	committeeInvoker := c.WithSigners(c.Committee)

	// Change fee, abort the transaction and check that contract cache wasn't persisted
	// for FAULTed tx at the same block.
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, committeeInvoker.Hash, setName, callflag.All, byte(transaction.ConflictsT), 5)
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	tx1 := committeeInvoker.PrepareInvocation(t, w.Bytes(), committeeInvoker.Signers)
	tx2 := committeeInvoker.PrepareInvoke(t, getName, byte(transaction.ConflictsT))
	committeeInvoker.AddNewBlock(t, tx1, tx2)
	committeeInvoker.CheckFault(t, tx1.Hash(), "ABORT")
	committeeInvoker.CheckHalt(t, tx2.Hash(), stackitem.Make(0))

	// Change fee and check that change is available for the next tx.
	tx1 = committeeInvoker.PrepareInvoke(t, setName, byte(transaction.ConflictsT), 5)
	tx2 = committeeInvoker.PrepareInvoke(t, getName, byte(transaction.ConflictsT))
	committeeInvoker.AddNewBlock(t, tx1, tx2)
	committeeInvoker.CheckHalt(t, tx1.Hash())
	committeeInvoker.CheckHalt(t, tx2.Hash(), stackitem.Make(5))
}

func TestPolicy_BlockedAccounts(t *testing.T) {
	c := newPolicyClient(t)
	e := c.Executor
	randomInvoker := c.WithSigners(c.NewAccount(t))
	committeeInvoker := c.WithSigners(c.Committee)
	unlucky := util.Uint160{1, 2, 3}

	t.Run("isBlocked", func(t *testing.T) {
		randomInvoker.Invoke(t, false, "isBlocked", unlucky)
	})

	t.Run("block-unblock account", func(t *testing.T) {
		committeeInvoker.Invoke(t, true, "blockAccount", unlucky)
		randomInvoker.Invoke(t, true, "isBlocked", unlucky)
		committeeInvoker.Invoke(t, true, "unblockAccount", unlucky)
		randomInvoker.Invoke(t, false, "isBlocked", unlucky)
	})

	t.Run("double-block", func(t *testing.T) {
		// block
		committeeInvoker.Invoke(t, true, "blockAccount", unlucky)

		// double-block should fail
		committeeInvoker.Invoke(t, false, "blockAccount", unlucky)

		// unblock
		committeeInvoker.Invoke(t, true, "unblockAccount", unlucky)

		// unblock the same account should fail as we don't have it blocked
		committeeInvoker.Invoke(t, false, "unblockAccount", unlucky)
	})

	t.Run("not signed by committee", func(t *testing.T) {
		randomInvoker.InvokeFail(t, "invalid committee signature", "blockAccount", unlucky)
		randomInvoker.InvokeFail(t, "invalid committee signature", "unblockAccount", unlucky)
	})

	t.Run("block-unblock contract", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "cannot block native contract", "blockAccount", c.NativeHash(t, nativenames.Neo))

		helper := neotest.CompileFile(t, c.CommitteeHash, "./helpers/policyhelper", "./helpers/policyhelper/policyhelper.yml")
		e.DeployContract(t, helper, nil)
		helperInvoker := e.CommitteeInvoker(helper.Hash)

		helperInvoker.Invoke(t, true, "do")
		committeeInvoker.Invoke(t, true, "blockAccount", helper.Hash)
		helperInvoker.InvokeFail(t, fmt.Sprintf("contract %s is blocked", helper.Hash.StringLE()), "do")

		committeeInvoker.Invoke(t, true, "unblockAccount", helper.Hash)
		helperInvoker.Invoke(t, true, "do")
	})
}
