package native_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
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

		switch name {
		case "GasPerBlock":
			// GasPerBlock is set on the next block
			c.CheckHalt(t, txGet.Hash(), stackitem.Make(defaultValue))
			c.AddNewBlock(t)
			randomInvoker.Invoke(t, defaultValue+1, getName)
		case "ExecFeeFactor":
			// ExecFeeFactor was risen, so the second transaction will fail because
			// of gas limit exceeding (its fees are out-of-date).
			c.CheckFault(t, txGet.Hash(), "gas limit exceeded")
			// Set in a separate block.
			committeeInvoker.Invoke(t, stackitem.Null{}, setName, defaultValue+1)
			// Get in the next block.
			randomInvoker.Invoke(t, defaultValue+1, getName)
		default:
			c.CheckHalt(t, txGet.Hash(), stackitem.Make(defaultValue+1))
			// Get in the next block.
			randomInvoker.Invoke(t, defaultValue+1, getName)
		}
	})
}

func testGetSetCache(t *testing.T, c *neotest.ContractInvoker, name string, defaultValue int64) {
	getName := "get" + name
	setName := "set" + name

	committeeInvoker := c.WithSigners(c.Committee)

	newVal := defaultValue - 1

	// Change fee, abort the transaction and check that contract cache wasn't persisted
	// for FAULTed tx at the same block.
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, committeeInvoker.Hash, setName, callflag.All, newVal)
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	tx1 := committeeInvoker.PrepareInvocation(t, w.Bytes(), committeeInvoker.Signers)
	tx2 := committeeInvoker.PrepareInvoke(t, getName)
	committeeInvoker.AddNewBlock(t, tx1, tx2)
	committeeInvoker.CheckFault(t, tx1.Hash(), "ABORT")
	committeeInvoker.CheckHalt(t, tx2.Hash(), stackitem.Make(defaultValue))

	// Change fee and check that change is available for the next tx.
	tx1 = committeeInvoker.PrepareInvoke(t, setName, newVal)
	tx2 = committeeInvoker.PrepareInvoke(t, getName)
	committeeInvoker.AddNewBlock(t, tx1, tx2)
	committeeInvoker.CheckHalt(t, tx1.Hash())
	if name != "GasPerBlock" {
		committeeInvoker.CheckHalt(t, tx2.Hash(), stackitem.Make(newVal))
	} else {
		committeeInvoker.CheckHalt(t, tx2.Hash(), stackitem.Make(defaultValue))
		committeeInvoker.Invoke(t, newVal, getName)
	}
}

func setNodesByRole(t *testing.T, designateInvoker *neotest.ContractInvoker, ok bool, r noderoles.Role, nodes keys.PublicKeys) {
	pubs := make([]any, len(nodes))
	for i := range nodes {
		pubs[i] = nodes[i].Bytes()
	}
	if ok {
		h := designateInvoker.Invoke(t, stackitem.Null{}, "designateAsRole", int64(r), pubs)
		designateInvoker.CheckTxNotificationEvent(t, h, 0, state.NotificationEvent{
			ScriptHash: designateInvoker.Hash,
			Name:       native.DesignationEventName,
			Item: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(int64(r)),
				stackitem.Make(designateInvoker.Chain.BlockHeight()),
			}),
		})
	} else {
		designateInvoker.InvokeFail(t, "", "designateAsRole", int64(r), pubs)
	}
}

func checkNodeRoles(t *testing.T, designateInvoker *neotest.ContractInvoker, ok bool, r noderoles.Role, index uint32, res keys.PublicKeys) {
	if ok {
		designateInvoker.InvokeAndCheck(t, func(t testing.TB, stack []stackitem.Item) {
			require.Equal(t, 1, len(stack))
			arr := stack[0].Value().([]stackitem.Item)
			require.Equal(t, len(res), len(arr))
			for i := range arr {
				require.Equal(t, res[i].Bytes(), arr[i].Value().([]byte), i)
			}
		}, "getDesignatedByRole", int64(r), int64(index))
	} else {
		designateInvoker.InvokeFail(t, "", "getDesignatedByRole", int64(r), int64(index))
	}
}
