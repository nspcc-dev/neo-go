package main

import (
	"io"
	"math/big"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestNEP5Balance(t *testing.T) {
	e := newExecutor(t, true)
	defer e.Close(t)
	cmd := []string{
		"neo-go", "wallet", "nep5", "balance",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", validatorWallet,
		"--addr", validatorAddr,
	}
	t.Run("NEO", func(t *testing.T) {
		b, index := e.Chain.GetGoverningTokenBalance(validatorHash)
		checkResult := func(t *testing.T) {
			e.checkNextLine(t, "^\\s*TokenHash:\\s*"+e.Chain.GoverningTokenHash().StringLE())
			e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+b.String())
			e.checkNextLine(t, "^\\s*Updated\\s*:\\s*"+strconv.FormatUint(uint64(index), 10))
			e.checkEOF(t)
		}
		t.Run("Alias", func(t *testing.T) {
			e.Run(t, append(cmd, "--token", "neo")...)
			checkResult(t)
		})
		t.Run("Hash", func(t *testing.T) {
			e.Run(t, append(cmd, "--token", e.Chain.GoverningTokenHash().StringLE())...)
			checkResult(t)
		})
	})
	t.Run("GAS", func(t *testing.T) {
		e.Run(t, append(cmd, "--token", "gas")...)
		e.checkNextLine(t, "^\\s*TokenHash:\\s*"+e.Chain.UtilityTokenHash().StringLE())
		b := e.Chain.GetUtilityTokenBalance(validatorHash)
		e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+util.Fixed8(b.Int64()).String())
	})
	t.Run("Invalid", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--token", "kek")...)
	})
	return
}

func TestNEP5Transfer(t *testing.T) {
	w, err := wallet.NewWalletFromFile("testdata/testwallet.json")
	require.NoError(t, err)
	defer w.Close()

	e := newExecutor(t, true)
	defer e.Close(t)
	args := []string{
		"neo-go", "wallet", "nep5", "transfer",
		"--unittest",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", validatorWallet,
		"--from", validatorAddr,
		"--to", w.Accounts[0].Address,
		"--token", "neo",
		"--amount", "1",
	}

	t.Run("InvalidPassword", func(t *testing.T) {
		e.In.WriteString("onetwothree\r")
		e.RunWithError(t, args...)
		e.In.Reset()
	})

	e.In.WriteString("one\r")
	e.Run(t, args...)
	e.checkTxPersisted(t)

	sh, err := address.StringToUint160(w.Accounts[0].Address)
	require.NoError(t, err)
	b, _ := e.Chain.GetGoverningTokenBalance(sh)
	require.Equal(t, big.NewInt(1), b)
}

func TestNEP5MultiTransfer(t *testing.T) {
	privs, _ := generateKeys(t, 3)

	e := newExecutor(t, true)
	defer e.Close(t)
	args := []string{
		"neo-go", "wallet", "nep5", "multitransfer",
		"--unittest", "--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", validatorWallet,
		"--from", validatorAddr,
		"neo:" + privs[0].Address() + ":42",
		"GAS:" + privs[1].Address() + ":7",
		client.NeoContractHash.StringLE() + ":" + privs[2].Address() + ":13",
	}

	e.In.WriteString("one\r")
	e.Run(t, args...)
	e.checkTxPersisted(t)

	b, _ := e.Chain.GetGoverningTokenBalance(privs[0].GetScriptHash())
	require.Equal(t, big.NewInt(42), b)
	b = e.Chain.GetUtilityTokenBalance(privs[1].GetScriptHash())
	require.Equal(t, big.NewInt(int64(util.Fixed8FromInt64(7))), b)
	b, _ = e.Chain.GetGoverningTokenBalance(privs[2].GetScriptHash())
	require.Equal(t, big.NewInt(13), b)
}

func TestNEP5ImportToken(t *testing.T) {
	e := newExecutor(t, true)
	defer e.Close(t)

	tmpDir := os.TempDir()
	walletPath := path.Join(tmpDir, "walletForImport.json")
	defer os.Remove(walletPath)

	e.Run(t, "neo-go", "wallet", "init", "--wallet", walletPath)
	e.Run(t, "neo-go", "wallet", "nep5", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", walletPath,
		"--token", client.GasContractHash.StringLE())
	e.Run(t, "neo-go", "wallet", "nep5", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", walletPath,
		"--token", client.NeoContractHash.StringLE())

	t.Run("Info", func(t *testing.T) {
		checkGASInfo := func(t *testing.T) {
			e.checkNextLine(t, "^Name:\\s*GAS")
			e.checkNextLine(t, "^Symbol:\\s*gas")
			e.checkNextLine(t, "^Hash:\\s*"+client.GasContractHash.StringLE())
			e.checkNextLine(t, "^Decimals:\\s*8")
			e.checkNextLine(t, "^Address:\\s*"+address.Uint160ToString(client.GasContractHash))
		}
		t.Run("WithToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep5", "info",
				"--wallet", walletPath, "--token", client.GasContractHash.StringLE())
			checkGASInfo(t)
		})
		t.Run("NoToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep5", "info",
				"--wallet", walletPath)
			checkGASInfo(t)
			_, err := e.Out.ReadString('\n')
			require.NoError(t, err)
			e.checkNextLine(t, "^Name:\\s*NEO")
			e.checkNextLine(t, "^Symbol:\\s*neo")
			e.checkNextLine(t, "^Hash:\\s*"+client.NeoContractHash.StringLE())
			e.checkNextLine(t, "^Decimals:\\s*0")
			e.checkNextLine(t, "^Address:\\s*"+address.Uint160ToString(client.NeoContractHash))
		})
		t.Run("Remove", func(t *testing.T) {
			e.In.WriteString("y\r")
			e.Run(t, "neo-go", "wallet", "nep5", "remove",
				"--wallet", walletPath, "--token", client.NeoContractHash.StringLE())
			e.Run(t, "neo-go", "wallet", "nep5", "info",
				"--wallet", walletPath)
			checkGASInfo(t)
			_, err := e.Out.ReadString('\n')
			require.Equal(t, err, io.EOF)
		})
	})
}
