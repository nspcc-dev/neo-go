package contract_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

var pathToInternalContracts = filepath.Join("..", "..", "..", "..", "internal", "contracts")

func TestGetCallFlags(t *testing.T) {
	bc, _ := chain.NewSingle(t)
	ic, err := bc.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
	require.NoError(t, err)

	ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, util.Uint160{1, 2, 3}, callflag.All)
	require.NoError(t, contract.GetCallFlags(ic))
	require.Equal(t, int64(callflag.All), ic.VM.Estack().Pop().Value().(*big.Int).Int64())
}

func TestCall(t *testing.T) {
	bc, _ := chain.NewSingle(t)
	ic, err := bc.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
	require.NoError(t, err)

	cs, currCs := contracts.GetTestContractState(t, pathToInternalContracts, 4, 5, random.Uint160()) // sender and IDs are not important for the test
	require.NoError(t, native.PutContractState(ic.DAO, cs))
	require.NoError(t, native.PutContractState(ic.DAO, currCs))

	currScript := currCs.NEF.Script
	h := cs.Hash

	addArgs := stackitem.NewArray([]stackitem.Item{stackitem.Make(1), stackitem.Make(2)})
	t.Run("Good", func(t *testing.T) {
		t.Run("2 arguments", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(addArgs)
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("add")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, big.NewInt(3), ic.VM.Estack().Pop().Value())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
		t.Run("3 arguments", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(
				append(addArgs.Value().([]stackitem.Item), stackitem.Make(3))))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("add")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, big.NewInt(6), ic.VM.Estack().Pop().Value())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
	})

	t.Run("CallExInvalidFlag", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(addArgs)
		ic.VM.Estack().PushVal(byte(0xFF))
		ic.VM.Estack().PushVal("add")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.Error(t, contract.Call(ic))
	})

	runInvalid := func(args ...any) func(t *testing.T) {
		return func(t *testing.T) {
			loadScriptWithHashAndFlags(ic, currScript, h, callflag.All, 42)
			for i := range args {
				ic.VM.Estack().PushVal(args[i])
			}
			// interops can both return error and panic,
			// we don't care which kind of error has occurred
			require.Panics(t, func() {
				err := contract.Call(ic)
				if err != nil {
					panic(err)
				}
			})
		}
	}

	t.Run("Invalid", func(t *testing.T) {
		t.Run("Hash", runInvalid(addArgs, "add", h.BytesBE()[1:]))
		t.Run("MissingHash", runInvalid(addArgs, "add", util.Uint160{}.BytesBE()))
		t.Run("Method", runInvalid(addArgs, stackitem.NewInterop("add"), h.BytesBE()))
		t.Run("MissingMethod", runInvalid(addArgs, "sub", h.BytesBE()))
		t.Run("DisallowedMethod", runInvalid(stackitem.NewArray(nil), "ret7", h.BytesBE()))
		t.Run("Arguments", runInvalid(1, "add", h.BytesBE()))
		t.Run("NotEnoughArguments", runInvalid(
			stackitem.NewArray([]stackitem.Item{stackitem.Make(1)}), "add", h.BytesBE()))
		t.Run("TooMuchArguments", runInvalid(
			stackitem.NewArray([]stackitem.Item{
				stackitem.Make(1), stackitem.Make(2), stackitem.Make(3), stackitem.Make(4)}),
			"add", h.BytesBE()))
	})

	t.Run("ReturnValues", func(t *testing.T) {
		t.Run("Many", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("invalidReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.Error(t, ic.VM.Run())
		})
		t.Run("Void", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("justReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, stackitem.Null{}, ic.VM.Estack().Pop().Item())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
	})

	t.Run("IsolatedStack", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray(nil))
		ic.VM.Estack().PushVal(callflag.All)
		ic.VM.Estack().PushVal("drop")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.Error(t, ic.VM.Run())
	})

	t.Run("CallInitialize", func(t *testing.T) {
		t.Run("Directly", runInvalid(stackitem.NewArray([]stackitem.Item{}), "_initialize", h.BytesBE()))

		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.Make(5)}))
		ic.VM.Estack().PushVal(callflag.All)
		ic.VM.Estack().PushVal("add3")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.NoError(t, ic.VM.Run())
		require.Equal(t, 2, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(8), ic.VM.Estack().Pop().Value())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
	})
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

func loadScript(ic *interop.Context, script []byte, args ...any) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithFlags(script, callflag.AllowCall)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}

func loadScriptWithHashAndFlags(ic *interop.Context, script []byte, hash util.Uint160, f callflag.CallFlag, args ...any) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithHash(script, hash, f)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}
