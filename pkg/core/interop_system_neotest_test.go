package core_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestSystemRuntimeGetRandom_DifferentTransactions(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetRandom)
	require.NoError(t, w.Err)
	script := w.Bytes()

	tx1 := e.PrepareInvocation(t, script, []neotest.Signer{e.Validator}, bc.BlockHeight()+1)
	tx2 := e.PrepareInvocation(t, script, []neotest.Signer{e.Validator}, bc.BlockHeight()+1)
	e.AddNewBlock(t, tx1, tx2)
	e.CheckHalt(t, tx1.Hash())
	e.CheckHalt(t, tx2.Hash())

	res1 := e.GetTxExecResult(t, tx1.Hash())
	res2 := e.GetTxExecResult(t, tx2.Hash())

	r1, err := res1.Stack[0].TryInteger()
	require.NoError(t, err)
	r2, err := res2.Stack[0].TryInteger()
	require.NoError(t, err)
	require.NotEqual(t, r1, r2)
}

func TestSystemContractCreateStandardAccount(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	w := io.NewBufBinWriter()

	t.Run("Good", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		pub := priv.PublicKey()

		emit.Bytes(w.BinWriter, pub.Bytes())
		emit.Syscall(w.BinWriter, interopnames.SystemContractCreateStandardAccount)
		require.NoError(t, w.Err)
		script := w.Bytes()

		tx := e.PrepareInvocation(t, script, []neotest.Signer{e.Validator}, bc.BlockHeight()+1)
		e.AddNewBlock(t, tx)
		e.CheckHalt(t, tx.Hash())

		res := e.GetTxExecResult(t, tx.Hash())
		value := res.Stack[0].Value().([]byte)
		u, err := util.Uint160DecodeBytesBE(value)
		require.NoError(t, err)
		require.Equal(t, pub.GetScriptHash(), u)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		w.Reset()
		emit.Bytes(w.BinWriter, []byte{1, 2, 3})
		emit.Syscall(w.BinWriter, interopnames.SystemContractCreateStandardAccount)
		require.NoError(t, w.Err)
		script := w.Bytes()

		tx := e.PrepareInvocation(t, script, []neotest.Signer{e.Validator}, bc.BlockHeight()+1)
		e.AddNewBlock(t, tx)
		e.CheckFault(t, tx.Hash(), "invalid prefix 1")
	})
}

func TestSystemContractCreateMultisigAccount(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	w := io.NewBufBinWriter()

	createScript := func(t *testing.T, pubs []interface{}, m int) []byte {
		w.Reset()
		emit.Array(w.BinWriter, pubs...)
		emit.Int(w.BinWriter, int64(m))
		emit.Syscall(w.BinWriter, interopnames.SystemContractCreateMultisigAccount)
		require.NoError(t, w.Err)
		return w.Bytes()
	}
	t.Run("Good", func(t *testing.T) {
		m, n := 3, 5
		pubs := make(keys.PublicKeys, n)
		arr := make([]interface{}, n)
		for i := range pubs {
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			pubs[i] = pk.PublicKey()
			arr[i] = pubs[i].Bytes()
		}
		script := createScript(t, arr, m)

		txH := e.InvokeScript(t, script, []neotest.Signer{acc})
		e.CheckHalt(t, txH)
		res := e.GetTxExecResult(t, txH)
		value := res.Stack[0].Value().([]byte)
		u, err := util.Uint160DecodeBytesBE(value)
		require.NoError(t, err)
		expected, err := smartcontract.CreateMultiSigRedeemScript(m, pubs)
		require.NoError(t, err)
		require.Equal(t, hash.Hash160(expected), u)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		script := createScript(t, []interface{}{[]byte{1, 2, 3}}, 1)
		e.InvokeScriptCheckFAULT(t, script, []neotest.Signer{acc}, "invalid prefix 1")
	})
	t.Run("Invalid m", func(t *testing.T) {
		pk, err := keys.NewPrivateKey()
		require.NoError(t, err)
		script := createScript(t, []interface{}{pk.PublicKey().Bytes()}, 2)
		e.InvokeScriptCheckFAULT(t, script, []neotest.Signer{acc}, "length of the signatures (2) is higher then the number of public keys")
	})
	t.Run("m overflows int32", func(t *testing.T) {
		pk, err := keys.NewPrivateKey()
		require.NoError(t, err)
		m := big.NewInt(math.MaxInt32)
		m.Add(m, big.NewInt(1))
		w.Reset()
		emit.Array(w.BinWriter, pk.Bytes())
		emit.BigInt(w.BinWriter, m)
		emit.Syscall(w.BinWriter, interopnames.SystemContractCreateMultisigAccount)
		require.NoError(t, w.Err)
		e.InvokeScriptCheckFAULT(t, w.Bytes(), []neotest.Signer{acc}, "m must be positive and fit int32")
	})
}

func TestSystemRuntimeGasLeft(t *testing.T) {
	const runtimeGasLeftPrice = 1 << 4

	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	w := io.NewBufBinWriter()

	gasLimit := 1100
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGasLeft)
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGasLeft)
	require.NoError(t, w.Err)
	tx := transaction.New(w.Bytes(), int64(gasLimit))
	tx.Nonce = neotest.Nonce()
	tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
	e.SignTx(t, tx, int64(gasLimit), acc)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash())
	res := e.GetTxExecResult(t, tx.Hash())
	l1 := res.Stack[0].Value().(*big.Int)
	l2 := res.Stack[1].Value().(*big.Int)

	require.Equal(t, int64(gasLimit-runtimeGasLeftPrice*interop.DefaultBaseExecFee), l1.Int64())
	require.Equal(t, int64(gasLimit-2*runtimeGasLeftPrice*interop.DefaultBaseExecFee), l2.Int64())
}

func TestLoadToken(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	managementInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Management))

	cs, _ := contracts.GetTestContractState(t, pathToInternalContracts, 0, 1, acc.ScriptHash())
	rawManifest, err := json.Marshal(cs.Manifest)
	require.NoError(t, err)
	rawNef, err := cs.NEF.Bytes()
	require.NoError(t, err)
	tx := managementInvoker.PrepareInvoke(t, "deploy", rawNef, rawManifest)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash())
	cInvoker := e.ValidatorInvoker(cs.Hash)

	t.Run("good", func(t *testing.T) {
		realBalance, _ := bc.GetGoverningTokenBalance(acc.ScriptHash())
		cInvoker.Invoke(t, stackitem.NewBigInteger(big.NewInt(realBalance.Int64()+1)), "callT0", acc.ScriptHash())
	})
	t.Run("invalid param count", func(t *testing.T) {
		cInvoker.InvokeFail(t, "method not found: callT2/1", "callT2", acc.ScriptHash())
	})
	t.Run("invalid contract", func(t *testing.T) {
		cInvoker.InvokeFail(t, "token contract 0000000000000000000000000000000000000000 not found: key not found", "callT1")
	})
}

func TestSystemRuntimeGetNetwork(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	w := io.NewBufBinWriter()

	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetNetwork)
	require.NoError(t, w.Err)
	e.InvokeScriptCheckHALT(t, w.Bytes(), []neotest.Signer{acc}, stackitem.NewBigInteger(big.NewInt(int64(bc.GetConfig().Magic))))
}

func TestSystemRuntimeGetAddressVersion(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	w := io.NewBufBinWriter()

	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetAddressVersion)
	require.NoError(t, w.Err)
	e.InvokeScriptCheckHALT(t, w.Bytes(), []neotest.Signer{acc}, stackitem.NewBigInteger(big.NewInt(int64(address.NEO3Prefix))))
}

func TestSystemRuntimeBurnGas(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	managementInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Management))

	cs, _ := contracts.GetTestContractState(t, pathToInternalContracts, 0, 1, acc.ScriptHash())
	rawManifest, err := json.Marshal(cs.Manifest)
	require.NoError(t, err)
	rawNef, err := cs.NEF.Bytes()
	require.NoError(t, err)
	tx := managementInvoker.PrepareInvoke(t, "deploy", rawNef, rawManifest)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash())
	cInvoker := e.ValidatorInvoker(cs.Hash)

	t.Run("good", func(t *testing.T) {
		h := cInvoker.Invoke(t, stackitem.Null{}, "burnGas", int64(1))
		res := e.GetTxExecResult(t, h)

		t.Run("gas limit exceeded", func(t *testing.T) {
			tx := e.NewUnsignedTx(t, cs.Hash, "burnGas", int64(2))
			e.SignTx(t, tx, res.GasConsumed, acc)
			e.AddNewBlock(t, tx)
			e.CheckFault(t, tx.Hash(), "GAS limit exceeded")
		})
	})
	t.Run("too big integer", func(t *testing.T) {
		gas := big.NewInt(math.MaxInt64)
		gas.Add(gas, big.NewInt(1))

		cInvoker.InvokeFail(t, "invalid GAS value", "burnGas", gas)
	})
	t.Run("zero GAS", func(t *testing.T) {
		cInvoker.InvokeFail(t, "GAS must be positive", "burnGas", int64(0))
	})
}

func TestSystemContractCreateAccount_Hardfork(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.ProtocolConfiguration) {
		c.P2PSigExtensions = true // `initBasicChain` requires Notary enabled
		c.Hardforks = map[string]uint32{
			config.HF2712FixSyscallFees.String(): 2,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	w := io.NewBufBinWriter()
	emit.Array(w.BinWriter, []interface{}{pub.Bytes(), pub.Bytes(), pub.Bytes()}...)
	emit.Int(w.BinWriter, int64(2))
	emit.Syscall(w.BinWriter, interopnames.SystemContractCreateMultisigAccount)
	require.NoError(t, w.Err)
	multisigScript := slice.Copy(w.Bytes())

	w.Reset()
	emit.Bytes(w.BinWriter, pub.Bytes())
	emit.Syscall(w.BinWriter, interopnames.SystemContractCreateStandardAccount)
	require.NoError(t, w.Err)
	standardScript := slice.Copy(w.Bytes())

	createAccTx := func(t *testing.T, script []byte) *transaction.Transaction {
		tx := e.PrepareInvocation(t, script, []neotest.Signer{e.Committee}, bc.BlockHeight()+1)
		return tx
	}

	// blocks #1, #2: old prices
	tx1Standard := createAccTx(t, standardScript)
	tx1Multisig := createAccTx(t, multisigScript)
	e.AddNewBlock(t, tx1Standard, tx1Multisig)
	e.CheckHalt(t, tx1Standard.Hash())
	e.CheckHalt(t, tx1Multisig.Hash())
	tx2Standard := createAccTx(t, standardScript)
	tx2Multisig := createAccTx(t, multisigScript)
	e.AddNewBlock(t, tx2Standard, tx2Multisig)
	e.CheckHalt(t, tx2Standard.Hash())
	e.CheckHalt(t, tx2Multisig.Hash())

	// block #3: updated prices (larger than the previous ones)
	tx3Standard := createAccTx(t, standardScript)
	tx3Multisig := createAccTx(t, multisigScript)
	e.AddNewBlock(t, tx3Standard, tx3Multisig)
	e.CheckHalt(t, tx3Standard.Hash())
	e.CheckHalt(t, tx3Multisig.Hash())
	require.True(t, tx1Standard.SystemFee == tx2Standard.SystemFee)
	require.True(t, tx1Multisig.SystemFee == tx2Multisig.SystemFee)
	require.True(t, tx2Standard.SystemFee < tx3Standard.SystemFee)
	require.True(t, tx2Multisig.SystemFee < tx3Multisig.SystemFee)
}

func TestSnapshotIsolation_Exceptions(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	// Contract A puts value in the storage, emits notifications and panics.
	srcA := `package contractA
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
			"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
			"github.com/nspcc-dev/neo-go/pkg/interop/storage"
		)
		func DoAndPanic(key, value []byte, nNtf int) int { // avoid https://github.com/nspcc-dev/neo-go/issues/2509
			c := storage.GetContext()
			storage.Put(c, key, value)
			for i := 0; i < nNtf; i++ {
				runtime.Notify("NotificationFromA", i)
			}
			panic("panic from A")
		}
		func CheckA(key []byte, nNtf int) bool {
			c := storage.GetContext()
			value := storage.Get(c, key)
			// If called from B, then no storage changes made by A should be visible by this moment (they have been discarded after exception handling).
			if value != nil {
				return false
			}
			notifications := runtime.GetNotifications(nil)
			if len(notifications) != nNtf {
				return false
			}
			// If called from B, then no notifications made by A should be visible by this moment (they have been discarded after exception handling).
			for i := 0; i < len(notifications); i++ {
				ntf := notifications[i]
				name := string(ntf[1].([]byte))
				if name == "NotificationFromA" {
					return false
				}
			}
			return true
		}
		func CheckB() bool {
			return contract.Call(runtime.GetCallingScriptHash(), "checkStorageChanges", contract.All).(bool)
		}`
	ctrA := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcA), &compiler.Options{
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Name:               "contractA",
		Permissions:        []manifest.Permission{{Methods: manifest.WildStrings{Value: nil}}},
	})
	e.DeployContract(t, ctrA, nil)

	var hashAStr string
	for i := 0; i < util.Uint160Size; i++ {
		hashAStr += fmt.Sprintf("%#x", ctrA.Hash[i])
		if i != util.Uint160Size-1 {
			hashAStr += ", "
		}
	}
	// Contract B puts value in the storage, emits notifications and calls A either
	// in try-catch block or without it. After that checks that proper notifications
	// and storage changes are available from different contexts.
	srcB := `package contractB
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop"
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
			"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
			"github.com/nspcc-dev/neo-go/pkg/interop/storage"
			"github.com/nspcc-dev/neo-go/pkg/interop/util"
		)
		var caughtKey = []byte("caught")
		func DoAndCatch(shouldRecover bool, keyA, valueA, keyB, valueB []byte, nNtfA, nNtfB1, nNtfB2 int) {
			if shouldRecover {
				defer func() {
					if r := recover(); r != nil {
						keyA := []byte("keyA") // defer can not capture variables from outside
						nNtfB1 := 2
						nNtfB2 := 4
						c := storage.GetContext()
						storage.Put(c, caughtKey, []byte{})
						for i := 0; i < nNtfB2; i++ {
							runtime.Notify("NotificationFromB after panic", i)
						}
						// Check that storage changes and notifications made by A are reverted.
						ok := contract.Call(interop.Hash160{` + hashAStr + `}, "checkA", contract.All, keyA, nNtfB1+nNtfB2).(bool)
						if !ok {
							util.Abort() // should never ABORT if snapshot isolation is correctly implemented.
						}
						// Check that storage changes made by B after catch are still available in current context.
						ok = CheckStorageChanges()
						if !ok {
							util.Abort() // should never ABORT if snapshot isolation is correctly implemented.
						}
						// Check that storage changes made by B after catch are still available from the outside context.
						ok = contract.Call(interop.Hash160{` + hashAStr + `}, "checkB", contract.All).(bool)
						if !ok {
							util.Abort() // should never ABORT if snapshot isolation is correctly implemented.
						}
					}
				}()
			}
			c := storage.GetContext()
			storage.Put(c, keyB, valueB)
			for i := 0; i < nNtfB1; i++ {
				runtime.Notify("NotificationFromB before panic", i)
			}
			contract.Call(interop.Hash160{` + hashAStr + `}, "doAndPanic", contract.All, keyA, valueA, nNtfA)
		}
		func CheckStorageChanges() bool {
			c := storage.GetContext()
			itm := storage.Get(c, caughtKey)
			return itm != nil
		}`
	ctrB := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcB), &compiler.Options{
		Name:               "contractB",
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Permissions:        []manifest.Permission{{Methods: manifest.WildStrings{Value: nil}}},
	})
	e.DeployContract(t, ctrB, nil)

	keyA := []byte("keyA")     // hard-coded in the contract code due to `defer` inability to capture variables from outside.
	valueA := []byte("valueA") // hard-coded in the contract code
	keyB := []byte("keyB")
	valueB := []byte("valueB")
	nNtfA := 3
	nNtfBBeforePanic := 2 // hard-coded in the contract code
	nNtfBAfterPanic := 4  // hard-coded in the contract code
	ctrInvoker := e.NewInvoker(ctrB.Hash, e.Committee)

	// Firstly, do not catch exception and check that all notifications are presented in the notifications list.
	h := ctrInvoker.InvokeFail(t, `unhandled exception: "panic from A"`, "doAndCatch", false, keyA, valueA, keyB, valueB, nNtfA, nNtfBBeforePanic, nNtfBAfterPanic)
	aer := e.GetTxExecResult(t, h)
	require.Equal(t, nNtfBBeforePanic+nNtfA, len(aer.Events))

	// Then catch exception thrown by A and check that only notifications/storage changes from B are saved.
	h = ctrInvoker.Invoke(t, stackitem.Null{}, "doAndCatch", true, keyA, valueA, keyB, valueB, nNtfA, nNtfBBeforePanic, nNtfBAfterPanic)
	aer = e.GetTxExecResult(t, h)
	require.Equal(t, nNtfBBeforePanic+nNtfBAfterPanic, len(aer.Events))
}

// This test is written to test nested calls with try-catch block and proper notifications handling.
func TestSnapshotIsolation_NestedContextException(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	srcA := `package contractA
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
			"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		)
		func CallA() {
			runtime.Notify("Calling A")
			contract.Call(runtime.GetExecutingScriptHash(), "a", contract.All)
			runtime.Notify("Finish")
		}
		func A() {
			defer func() {
				if r := recover(); r != nil {
					runtime.Notify("Caught")
				}
			}()
			runtime.Notify("A")
			contract.Call(runtime.GetExecutingScriptHash(), "b", contract.All)
			runtime.Notify("Unreachable A")
		}
		func B() int {
			runtime.Notify("B")
			contract.Call(runtime.GetExecutingScriptHash(), "c", contract.All)
			runtime.Notify("Unreachable B")
			return 5
		}
		func C() {
			runtime.Notify("C")
			panic("exception from C")
		}`
	ctrA := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcA), &compiler.Options{
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Name:               "contractA",
		Permissions:        []manifest.Permission{{Methods: manifest.WildStrings{Value: nil}}},
	})
	e.DeployContract(t, ctrA, nil)

	ctrInvoker := e.NewInvoker(ctrA.Hash, e.Committee)
	h := ctrInvoker.Invoke(t, stackitem.Null{}, "callA")
	aer := e.GetTxExecResult(t, h)
	require.Equal(t, 4, len(aer.Events))
	require.Equal(t, "Calling A", aer.Events[0].Name)
	require.Equal(t, "A", aer.Events[1].Name)
	require.Equal(t, "Caught", aer.Events[2].Name)
	require.Equal(t, "Finish", aer.Events[3].Name)
}

// This test is written to avoid https://github.com/neo-project/neo/issues/2746.
func TestSnapshotIsolation_CallToItself(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	// Contract A calls method of self and throws if storage changes made by Do are unavailable after call to it.
	srcA := `package contractA
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
			"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
			"github.com/nspcc-dev/neo-go/pkg/interop/storage"
		)
		var key = []byte("key")
		func Test() {
			contract.Call(runtime.GetExecutingScriptHash(), "callMyselfAndCheck", contract.All)
		}
		func CallMyselfAndCheck() {
			contract.Call(runtime.GetExecutingScriptHash(), "do", contract.All)
			c := storage.GetContext()
			val := storage.Get(c, key)
			if val == nil {
				panic("changes from previous context were not persisted")
			}
		}
		func Do() {
			c := storage.GetContext()
			storage.Put(c, key, []byte("value"))
		}
		func Check() {
			c := storage.GetContext()
			val := storage.Get(c, key)
			if val == nil {
				panic("value is nil")
			}
		}
`
	ctrA := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcA), &compiler.Options{
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Name:               "contractA",
		Permissions:        []manifest.Permission{{Methods: manifest.WildStrings{Value: nil}}},
	})
	e.DeployContract(t, ctrA, nil)

	ctrInvoker := e.NewInvoker(ctrA.Hash, e.Committee)
	ctrInvoker.Invoke(t, stackitem.Null{}, "test")

	// A separate call is needed to check whether all VM contexts were properly
	// unwrapped and persisted during the previous call.
	ctrInvoker.Invoke(t, stackitem.Null{}, "check")
}

// This test is written to check https://github.com/nspcc-dev/neo-go/issues/2509
// and https://github.com/neo-project/neo/pull/2745#discussion_r879167180.
func TestRET_after_FINALLY_PanicInsideVoidMethod(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	// Contract A throws catchable exception. It also has a non-void method.
	srcA := `package contractA
		func Panic() {
			panic("panic from A")
		}
		func ReturnSomeValue() int {
			return 5
		}`
	ctrA := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcA), &compiler.Options{
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Name:               "contractA",
	})
	e.DeployContract(t, ctrA, nil)

	var hashAStr string
	for i := 0; i < util.Uint160Size; i++ {
		hashAStr += fmt.Sprintf("%#x", ctrA.Hash[i])
		if i != util.Uint160Size-1 {
			hashAStr += ", "
		}
	}
	// Contract B calls A and catches the exception thrown by A.
	srcB := `package contractB
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop"
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
		)
		func Catch() {
			defer func() {
				if r := recover(); r != nil {
					// Call method with return value to check https://github.com/neo-project/neo/pull/2745#discussion_r879167180.
					contract.Call(interop.Hash160{` + hashAStr + `}, "returnSomeValue", contract.All)
				}
			}()
			contract.Call(interop.Hash160{` + hashAStr + `}, "panic", contract.All)
		}`
	ctrB := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcB), &compiler.Options{
		Name:               "contractB",
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Permissions: []manifest.Permission{
			{
				Methods: manifest.WildStrings{Value: nil},
			},
		},
	})
	e.DeployContract(t, ctrB, nil)

	ctrInvoker := e.NewInvoker(ctrB.Hash, e.Committee)
	ctrInvoker.Invoke(t, stackitem.Null{}, "catch")
}

// This test is written to check https://github.com/neo-project/neo/pull/2745#discussion_r879125733.
func TestRET_after_FINALLY_CallNonVoidAfterVoidMethod(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	// Contract A has two methods. One of them has no return value, and the other has it.
	srcA := `package contractA
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func NoRet() {
			runtime.Notify("no ret")
		}
		func HasRet() int {
			runtime.Notify("ret")
			return 5
		}`
	ctrA := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcA), &compiler.Options{
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Name:               "contractA",
	})
	e.DeployContract(t, ctrA, nil)

	var hashAStr string
	for i := 0; i < util.Uint160Size; i++ {
		hashAStr += fmt.Sprintf("%#x", ctrA.Hash[i])
		if i != util.Uint160Size-1 {
			hashAStr += ", "
		}
	}
	// Contract B calls A in try-catch block.
	srcB := `package contractB
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop"
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
			"github.com/nspcc-dev/neo-go/pkg/interop/util"
		)
		func CallAInTryCatch() {
			defer func() {
				if r := recover(); r != nil {
					util.Abort() // should never happen
				}
			}()
			contract.Call(interop.Hash160{` + hashAStr + `}, "noRet", contract.All)
			contract.Call(interop.Hash160{` + hashAStr + `}, "hasRet", contract.All)
		}`
	ctrB := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcB), &compiler.Options{
		Name:               "contractB",
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Permissions: []manifest.Permission{
			{
				Methods: manifest.WildStrings{Value: nil},
			},
		},
	})
	e.DeployContract(t, ctrB, nil)

	ctrInvoker := e.NewInvoker(ctrB.Hash, e.Committee)
	h := ctrInvoker.Invoke(t, stackitem.Null{}, "callAInTryCatch")
	aer := e.GetTxExecResult(t, h)

	require.Equal(t, 1, len(aer.Stack))
}

// This test is created to check https://github.com/neo-project/neo/pull/2755#discussion_r880087983.
func TestCALLL_from_VoidContext(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	// Contract A has void method `CallHasRet` which calls non-void method `HasRet`.
	srcA := `package contractA
		func CallHasRet() { // Creates a context with non-nil onUnload.
			HasRet()
		}
		func HasRet() int { // CALL_L clones parent context, check that onUnload is not cloned.
			return 5
		}`
	ctrA := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(srcA), &compiler.Options{
		NoEventsCheck:      true,
		NoPermissionsCheck: true,
		Name:               "contractA",
	})
	e.DeployContract(t, ctrA, nil)

	ctrInvoker := e.NewInvoker(ctrA.Hash, e.Committee)
	ctrInvoker.Invoke(t, stackitem.Null{}, "callHasRet")
}
