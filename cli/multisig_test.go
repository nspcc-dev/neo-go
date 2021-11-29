package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

// Test signing of multisig transactions.
// 1. Transfer funds to a created multisig address.
// 2. Transfer from multisig to another account.
func TestSignMultisigTx(t *testing.T) {
	e := newExecutor(t, true)

	privs, pubs := generateKeys(t, 3)
	script, err := smartcontract.CreateMultiSigRedeemScript(2, pubs)
	require.NoError(t, err)
	multisigHash := hash.Hash160(script)
	multisigAddr := address.Uint160ToString(multisigHash)

	// Create 2 wallets participating in multisig.
	tmpDir := t.TempDir()
	wallet1Path := filepath.Join(tmpDir, "multiWallet1.json")
	wallet2Path := filepath.Join(tmpDir, "multiWallet2.json")

	addAccount := func(w string, wif string) {
		e.Run(t, "neo-go", "wallet", "init", "--wallet", w)
		e.In.WriteString("acc\rpass\rpass\r")
		e.Run(t, "neo-go", "wallet", "import-multisig",
			"--wallet", w,
			"--wif", wif,
			"--min", "2",
			hex.EncodeToString(pubs[0].Bytes()),
			hex.EncodeToString(pubs[1].Bytes()),
			hex.EncodeToString(pubs[2].Bytes()))
	}
	addAccount(wallet1Path, privs[0].WIF())
	addAccount(wallet2Path, privs[1].WIF())

	// Transfer funds to the multisig.
	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "nep17", "multitransfer",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--from", validatorAddr,
		"--force",
		"NEO:"+multisigAddr+":4",
		"GAS:"+multisigAddr+":1")
	e.checkTxPersisted(t)

	// Sign and transfer funds to another account.
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	txPath := filepath.Join(tmpDir, "multisigtx.json")
	t.Cleanup(func() {
		os.Remove(txPath)
	})
	e.In.WriteString("pass\r")
	e.Run(t, "neo-go", "wallet", "nep17", "transfer",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wallet1Path, "--from", multisigAddr,
		"--to", priv.Address(), "--token", "NEO", "--amount", "1",
		"--out", txPath)

	simplePriv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	{ // Simple signer, not in signers.
		e.In.WriteString("acc\rpass\rpass\r")
		e.Run(t, "neo-go", "wallet", "import",
			"--wallet", wallet1Path,
			"--wif", simplePriv.WIF())
		t.Run("sign with missing signer", func(t *testing.T) {
			e.In.WriteString("pass\r")
			e.RunWithError(t, "neo-go", "wallet", "sign",
				"--wallet", wallet1Path, "--address", simplePriv.Address(),
				"--in", txPath, "--out", txPath)
		})
	}

	// missing address
	e.RunWithError(t, "neo-go", "wallet", "sign",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wallet2Path,
		"--in", txPath, "--out", txPath)

	t.Run("test invoke", func(t *testing.T) {
		t.Run("missing file", func(t *testing.T) {
			e.RunWithError(t, "neo-go", "util", "txdump")
			fmt.Println(e.Out.String())
		})

		t.Run("no invoke", func(t *testing.T) {
			e.Run(t, "neo-go", "util", "txdump", txPath)
			e.checkTxTestInvokeOutput(t, 11)
			e.checkEOF(t)
		})

		e.Run(t, "neo-go", "util", "txdump",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			txPath)
		e.checkTxTestInvokeOutput(t, 11)
		res := new(result.Invoke)
		require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
		require.Equal(t, vm.HaltState.String(), res.State, res.FaultException)
	})

	e.In.WriteString("pass\r")
	e.Run(t, "neo-go", "wallet", "sign",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wallet2Path, "--address", multisigAddr,
		"--in", txPath, "--out", txPath)
	e.checkTxPersisted(t)

	b, _ := e.Chain.GetGoverningTokenBalance(priv.GetScriptHash())
	require.Equal(t, big.NewInt(1), b)
	b, _ = e.Chain.GetGoverningTokenBalance(multisigHash)
	require.Equal(t, big.NewInt(3), b)

	t.Run("via invokefunction", func(t *testing.T) {
		h := deployVerifyContract(t, e)

		e.In.WriteString("acc\rpass\rpass\r")
		e.Run(t, "neo-go", "wallet", "import-deployed",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", wallet1Path, "--wif", simplePriv.WIF(),
			"--contract", h.StringLE())

		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "contract", "invokefunction",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", wallet1Path, "--address", multisigHash.StringLE(), // test with scripthash instead of address
			"--out", txPath,
			e.Chain.GoverningTokenHash().StringLE(), "transfer",
			"bytes:"+multisigHash.StringBE(),
			"bytes:"+priv.GetScriptHash().StringBE(),
			"int:1", "bytes:",
			"--", multisigHash.StringLE()+":"+"Global",
			h.StringLE(),
			simplePriv.GetScriptHash().StringLE(),
		)

		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign",
			"--wallet", wallet2Path, "--address", multisigAddr,
			"--in", txPath, "--out", txPath)

		// Simple signer, not in signers.
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign",
			"--wallet", wallet1Path, "--address", simplePriv.Address(),
			"--in", txPath, "--out", txPath)

		// Contract.
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", wallet1Path, "--address", address.Uint160ToString(h),
			"--in", txPath, "--out", txPath)
		tx, _ := e.checkTxPersisted(t)
		require.Equal(t, 3, len(tx.Signers))

		b, _ := e.Chain.GetGoverningTokenBalance(priv.GetScriptHash())
		require.Equal(t, big.NewInt(2), b)
		b, _ = e.Chain.GetGoverningTokenBalance(multisigHash)
		require.Equal(t, big.NewInt(2), b)
	})
}

func (e *executor) checkTxTestInvokeOutput(t *testing.T, scriptSize int) {
	e.checkNextLine(t, `Hash:\s+`)
	e.checkNextLine(t, `OnChain:\s+false`)
	e.checkNextLine(t, `ValidUntil:\s+\d+`)
	e.checkNextLine(t, `Signer:\s+\w+`)
	e.checkNextLine(t, `SystemFee:\s+(\d|\.)+`)
	e.checkNextLine(t, `NetworkFee:\s+(\d|\.)+`)
	e.checkNextLine(t, `Script:\s+\w+`)
	e.checkScriptDump(t, scriptSize)
}

func (e *executor) checkScriptDump(t *testing.T, scriptSize int) {
	e.checkNextLine(t, `INDEX\s+`)
	for i := 0; i < scriptSize; i++ {
		e.checkNextLine(t, `\d+\s+\w+`)
	}
}
