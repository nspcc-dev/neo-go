package contract_test

import (
	"bytes"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/stretchr/testify/require"
)

func TestCreateStandardAccount(t *testing.T) {
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

func TestCreateMultisigAccount(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	w := io.NewBufBinWriter()

	createScript := func(t *testing.T, pubs []any, m int) []byte {
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
		arr := make([]any, n)
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
		script := createScript(t, []any{[]byte{1, 2, 3}}, 1)
		e.InvokeScriptCheckFAULT(t, script, []neotest.Signer{acc}, "invalid prefix 1")
	})
	t.Run("Invalid m", func(t *testing.T) {
		pk, err := keys.NewPrivateKey()
		require.NoError(t, err)
		script := createScript(t, []any{pk.PublicKey().Bytes()}, 2)
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

func TestCreateAccount_HFAspidochelone(t *testing.T) {
	const enabledHeight = 3
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.Hardforks = map[string]uint32{
			config.HFAspidochelone.String(): enabledHeight,
			config.HFBasilisk.String():      enabledHeight,
			config.HFCockatrice.String():    enabledHeight,
			config.HFDomovoi.String():       enabledHeight,
			config.HFEchidna.String():       enabledHeight,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	w := io.NewBufBinWriter()
	emit.Array(w.BinWriter, []any{pub.Bytes(), pub.Bytes(), pub.Bytes()}...)
	emit.Int(w.BinWriter, int64(2))
	emit.Syscall(w.BinWriter, interopnames.SystemContractCreateMultisigAccount)
	require.NoError(t, w.Err)
	multisigScript := bytes.Clone(w.Bytes())

	w.Reset()
	emit.Bytes(w.BinWriter, pub.Bytes())
	emit.Syscall(w.BinWriter, interopnames.SystemContractCreateStandardAccount)
	require.NoError(t, w.Err)
	standardScript := bytes.Clone(w.Bytes())

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
	require.Equal(t, uint32(enabledHeight-1), e.Chain.BlockHeight())
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
