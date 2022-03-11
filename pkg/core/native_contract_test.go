package core_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestNativeContract_Invoke(t *testing.T) {
	const (
		transferCPUFee          = 1 << 17
		transferStorageFee      = 50
		systemContractCallPrice = 1 << 15
	)
	bc, validator, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validator, committee)
	gasHash := e.NativeHash(t, nativenames.Gas)

	baseExecFee := bc.GetBaseExecFee()
	price := fee.Opcode(baseExecFee, opcode.SYSCALL, // System.Contract.Call
		opcode.PUSHDATA1, // contract hash (20 byte)
		opcode.PUSHDATA1, // method
		opcode.PUSH15,    // call flags
		// `transfer` args:
		opcode.PUSHDATA1, // from
		opcode.PUSHDATA1, // to
		opcode.PUSH1,     // amount
		opcode.PUSHNULL,  // data
		// end args
		opcode.PUSH4, // amount of args
		opcode.PACK,  // pack args
	)
	price += systemContractCallPrice*baseExecFee + // System.Contract.Call price
		transferCPUFee*baseExecFee + // `transfer` itself
		transferStorageFee*bc.GetStoragePrice() // `transfer` storage price

	tx := e.NewUnsignedTx(t, gasHash, "transfer", validator.ScriptHash(), validator.ScriptHash(), 1, nil)
	e.SignTx(t, tx, -1, validator)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash(), stackitem.Make(true))

	// Enough for Call and other opcodes, but not enough for "transfer" call.
	tx = e.NewUnsignedTx(t, gasHash, "transfer", validator.ScriptHash(), validator.ScriptHash(), 1, nil)
	e.SignTx(t, tx, price-1, validator)
	e.AddNewBlock(t, tx)
	e.CheckFault(t, tx.Hash(), "gas limit exceeded")
}

func TestNativeContract_InvokeInternal(t *testing.T) {
	bc, validator, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validator, committee)
	clState := bc.GetContractState(e.NativeHash(t, nativenames.CryptoLib))
	require.NotNil(t, clState)
	md := clState.Manifest.ABI.GetMethod("ripemd160", 1)
	require.NotNil(t, md)

	t.Run("fail, bad current script hash", func(t *testing.T) {
		ic := bc.GetTestVM(trigger.Application, nil, nil)
		v := ic.SpawnVM()
		fakeH := util.Uint160{1, 2, 3}
		v.LoadScriptWithHash(clState.NEF.Script, fakeH, callflag.All)
		input := []byte{1, 2, 3, 4}
		v.Estack().PushVal(input)
		v.Context().Jump(md.Offset)

		// Bad current script hash
		err := v.Run()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), fmt.Sprintf("native contract %s (version 0) not found", fakeH.StringLE())), err.Error())
	})

	t.Run("fail, bad NativeUpdateHistory height", func(t *testing.T) {
		bcBad, validatorBad, committeeBad := chain.NewMultiWithCustomConfig(t, func(c *config.ProtocolConfiguration) {
			c.NativeUpdateHistories = map[string][]uint32{
				nativenames.Policy:      {0},
				nativenames.Neo:         {0},
				nativenames.Gas:         {0},
				nativenames.Designation: {0},
				nativenames.StdLib:      {0},
				nativenames.Management:  {0},
				nativenames.Oracle:      {0},
				nativenames.Ledger:      {0},
				nativenames.CryptoLib:   {1},
			}
		})
		eBad := neotest.NewExecutor(t, bcBad, validatorBad, committeeBad)

		ic := bcBad.GetTestVM(trigger.Application, nil, nil)
		v := ic.SpawnVM()
		v.LoadScriptWithHash(clState.NEF.Script, clState.Hash, callflag.All) // hash is not affected by native update history
		input := []byte{1, 2, 3, 4}
		v.Estack().PushVal(input)
		v.Context().Jump(md.Offset)

		// It's prohibited to call natives before NativeUpdateHistory[0] height.
		err := v.Run()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "native contract CryptoLib is active after height = 1"))

		// Add new block => CryptoLib should be active now.
		eBad.AddNewBlock(t)
		ic = bcBad.GetTestVM(trigger.Application, nil, nil)
		v = ic.SpawnVM()
		v.LoadScriptWithHash(clState.NEF.Script, clState.Hash, callflag.All) // hash is not affected by native update history
		v.Estack().PushVal(input)
		v.Context().Jump(md.Offset)

		require.NoError(t, v.Run())
		value := v.Estack().Pop().Bytes()
		require.Equal(t, hash.RipeMD160(input).BytesBE(), value)
	})

	t.Run("success", func(t *testing.T) {
		ic := bc.GetTestVM(trigger.Application, nil, nil)
		v := ic.SpawnVM()
		v.LoadScriptWithHash(clState.NEF.Script, clState.Hash, callflag.All)
		input := []byte{1, 2, 3, 4}
		v.Estack().PushVal(input)
		v.Context().Jump(md.Offset)

		require.NoError(t, v.Run())

		value := v.Estack().Pop().Bytes()
		require.Equal(t, hash.RipeMD160(input).BytesBE(), value)
	})
}

func TestNativeContract_InvokeOtherContract(t *testing.T) {
	bc, validator, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validator, committee)
	managementInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Management))
	gasInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas))

	cs, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, validator.ScriptHash())
	cs.Hash = state.CreateContractHash(validator.ScriptHash(), cs.NEF.Checksum, cs.Manifest.Name) // set proper hash
	manifB, err := json.Marshal(cs.Manifest)
	require.NoError(t, err)
	nefB, err := cs.NEF.Bytes()
	require.NoError(t, err)
	si, err := cs.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nefB, manifB)

	t.Run("non-native, no return", func(t *testing.T) {
		// `onNEP17Payment` will be invoked on test contract from GAS contract.
		gasInvoker.Invoke(t, true, "transfer", validator.ScriptHash(), cs.Hash, 1, nil)
	})
}
