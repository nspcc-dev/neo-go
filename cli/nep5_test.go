package main

import (
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	h, err := util.Uint256DecodeStringLE(strings.TrimSpace(line))
	require.NoError(t, err, "can't decode tx hash: %s", line)

	tx := e.GetTransaction(t, h)
	aer, err := e.Chain.GetAppExecResult(tx.Hash())
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, aer.VMState)

	sh, err := address.StringToUint160(w.Accounts[0].Address)
	require.NoError(t, err)
	b, _ := e.Chain.GetGoverningTokenBalance(sh)
	require.Equal(t, big.NewInt(1), b)
}
