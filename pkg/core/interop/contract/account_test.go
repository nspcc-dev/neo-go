package contract_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"slices"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
		e.InvokeScriptCheckFAULT(t, script, []neotest.Signer{acc}, "invalid multisig parameters: m=2, public keys count=1")
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
		e.InvokeScriptCheckFAULT(t, w.Bytes(), []neotest.Signer{acc}, "m is out of int32 range")
	})
	t.Run("negative m price before Huyao", func(t *testing.T) {
		bc, _ := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
			c.Hardforks = map[string]uint32{config.HFGorgon.String(): 0}
		})
		pub, err := hex.DecodeString("036b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c296")
		require.NoError(t, err)
		w := io.NewBufBinWriter()
		emit.Bytes(w.BinWriter, pub)
		emit.Opcodes(w.BinWriter, opcode.PUSH1, opcode.PACK, opcode.PUSHM1)
		syscallOffset := w.Len()
		emit.Syscall(w.BinWriter, interopnames.SystemContractCreateMultisigAccount)
		emit.Opcodes(w.BinWriter, opcode.RET)
		require.NoError(t, w.Err)

		ic, err := bc.GetTestVM(trigger.Application, nil, nil)
		require.NoError(t, err)
		defer ic.Finalize()
		ic.VM.LoadScriptWithFlags(w.Bytes(), callflag.All)

		for ic.VM.Context().NextIP() < syscallOffset {
			require.NoError(t, ic.VM.Step())
		}
		gasBefore := ic.VM.GasConsumed()

		require.ErrorContains(t, ic.VM.Run(), "invalid multisig parameters: m=-1, public keys count=1")
		require.True(t, ic.VM.HasFailed())
		require.Zero(t, ic.VM.GasConsumed()-gasBefore)
	})
	t.Run("parameters are checked before decoding keys", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			m    int
			n    int
		}{
			{"zero m", 0, 1},
			{"too many keys", 1, smartcontract.MaxMultiSigRedeemScriptKeys + 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ic, err := bc.GetTestVM(trigger.Application, nil, nil)
				require.NoError(t, err)
				defer ic.Finalize()
				ic.VM.Estack().PushVal(stackitem.NewArray(slices.Repeat([]stackitem.Item{stackitem.NewByteArray([]byte{0})}, tc.n)))
				ic.VM.Estack().PushVal(tc.m)
				require.EqualError(t, contract.CreateMultisigAccount(ic), fmt.Sprintf("invalid multisig parameters: m=%d, public keys count=%d", tc.m, tc.n))
				require.Zero(t, ic.VM.GasConsumed())
			})
		}
	})
	t.Run("too many valid keys", func(t *testing.T) {
		pk, err := keys.NewPrivateKey()
		require.NoError(t, err)
		script := createScript(t, slices.Repeat([]any{pk.PublicKey().Bytes()}, smartcontract.MaxMultiSigRedeemScriptKeys+1), 1)
		e.InvokeScriptCheckFAULT(t, script, []neotest.Signer{acc}, "invalid multisig parameters: m=1, public keys count=1025")
	})
	t.Run("fee is charged before decoding keys", func(t *testing.T) {
		ic, err := bc.GetTestVM(trigger.Application, nil, nil)
		require.NoError(t, err)
		defer ic.Finalize()
		ic.VM.SetGasLimit(0)
		ic.VM.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{0})}))
		ic.VM.Estack().PushVal(1)
		require.ErrorIs(t, contract.CreateMultisigAccount(ic), vm.ErrGASLimitExceeded)
	})
}

func TestCreateMultisigAccount_GasConsumed(t *testing.T) {
	pub := make([]byte, smartcontract.PublicKeyLen)

	emitScript := func(t *testing.T, pubKeys []any, m int) ([]byte, int) {
		w := io.NewBufBinWriter()
		emit.Array(w.BinWriter, pubKeys...)
		emit.Int(w.BinWriter, int64(m))
		syscallOffset := w.Len()
		emit.Syscall(w.BinWriter, interopnames.SystemContractCreateMultisigAccount)
		require.NoError(t, w.Err)
		return w.Bytes(), syscallOffset
	}

	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	for _, tc := range []struct {
		name    string
		pubKeys []any
		m       int
		err     string
	}{
		{
			name:    "m < 1",
			pubKeys: []any{pub},
			m:       0,
			err:     "invalid multisig parameters: m=0, public keys count=1",
		},
		{
			name:    "m > n",
			pubKeys: []any{pub},
			m:       2,
			err:     "invalid multisig parameters: m=2, public keys count=1",
		},
		{
			name:    "too many keys",
			pubKeys: slices.Repeat([]any{pub}, smartcontract.MaxMultiSigRedeemScriptKeys+1),
			m:       1,
			err:     "invalid multisig parameters: m=1, public keys count=1025",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script, syscallOffset := emitScript(t, tc.pubKeys, tc.m)
			baseFee := bc.GetBaseExecFee()
			var expected int64
			ctx := vm.NewContext(script)
			for ctx.NextIP() < syscallOffset {
				op, _, err := ctx.Next()
				require.NoError(t, err)
				expected += fee.Opcode(baseFee, op)
			}
			h := e.InvokeScriptCheckFAULT(t, script, []neotest.Signer{acc}, tc.err)
			require.Equal(t, vm.PicoGasToDatoshiInt64(expected), e.GetTxExecResult(t, h).GasConsumed)
		})
	}
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
