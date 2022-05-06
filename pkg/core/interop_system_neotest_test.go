package core_test

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
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
