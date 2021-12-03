package native_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

func newNativeClient(t *testing.T, name string) *neotest.ContractInvoker {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	return e.CommitteeInvoker(e.NativeHash(t, name))
}

func testGetSet(t *testing.T, c *neotest.ContractInvoker, name string, defaultValue, minValue, maxValue int64) {
	getName := "get" + name
	setName := "set" + name

	randomInvoker := c.WithSigners(c.NewAccount(t))
	committeeInvoker := c.WithSigners(c.Committee)

	t.Run("set, not signed by committee", func(t *testing.T) {
		randomInvoker.InvokeFail(t, "invalid committee signature", setName, minValue+1)
	})
	t.Run("get, default value", func(t *testing.T) {
		randomInvoker.Invoke(t, defaultValue, getName)
	})
	t.Run("set, too small value", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "", setName, minValue-1)
	})

	if maxValue != 0 {
		t.Run("set, too large value", func(t *testing.T) {
			// use big.Int because max can be `math.MaxInt64`
			max := big.NewInt(maxValue)
			max.Add(max, big.NewInt(1))
			committeeInvoker.InvokeFail(t, "", setName, max)
		})
	}

	t.Run("set, success", func(t *testing.T) {
		// Set and get in the same block.
		txSet := committeeInvoker.PrepareInvoke(t, setName, defaultValue+1)
		txGet := randomInvoker.PrepareInvoke(t, getName)
		c.AddNewBlock(t, txSet, txGet)
		c.CheckHalt(t, txSet.Hash(), stackitem.Null{})

		if name != "GasPerBlock" { // GasPerBlock is set on the next block
			c.CheckHalt(t, txGet.Hash(), stackitem.Make(defaultValue+1))
		} else {
			c.CheckHalt(t, txGet.Hash(), stackitem.Make(defaultValue))
			c.AddNewBlock(t)
			randomInvoker.Invoke(t, defaultValue+1, getName)
		}

		// Get in the next block.
		randomInvoker.Invoke(t, defaultValue+1, getName)
	})
}
