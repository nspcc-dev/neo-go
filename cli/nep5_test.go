package main

import (
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
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
