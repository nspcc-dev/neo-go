package main

import (
	"io"
	"math/big"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestNEP17Balance(t *testing.T) {
	e := newExecutor(t, true)
	cmdbalance := []string{"neo-go", "wallet", "nep17", "balance"}
	cmdbase := append(cmdbalance,
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
	)
	cmd := append(cmdbase, "--address", validatorAddr)
	t.Run("NEO", func(t *testing.T) {
		b, index := e.Chain.GetGoverningTokenBalance(validatorHash)
		checkResult := func(t *testing.T) {
			e.checkNextLine(t, "^\\s*Account\\s+"+validatorAddr)
			e.checkNextLine(t, "^\\s*NEO:\\s+NeoToken \\("+e.Chain.GoverningTokenHash().StringLE()+"\\)")
			e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+b.String()+"$")
			e.checkNextLine(t, "^\\s*Updated\\s*:\\s*"+strconv.FormatUint(uint64(index), 10))
			e.checkEOF(t)
		}
		t.Run("Alias", func(t *testing.T) {
			e.Run(t, append(cmd, "--token", "NEO")...)
			checkResult(t)
		})
		t.Run("Hash", func(t *testing.T) {
			e.Run(t, append(cmd, "--token", e.Chain.GoverningTokenHash().StringLE())...)
			checkResult(t)
		})
	})
	t.Run("GAS", func(t *testing.T) {
		e.Run(t, append(cmd, "--token", "GAS")...)
		e.checkNextLine(t, "^\\s*Account\\s+"+validatorAddr)
		e.checkNextLine(t, "^\\s*GAS:\\s+GasToken \\("+e.Chain.UtilityTokenHash().StringLE()+"\\)")
		b := e.Chain.GetUtilityTokenBalance(validatorHash)
		e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+fixedn.Fixed8(b.Int64()).String()+"$")
	})
	t.Run("all accounts", func(t *testing.T) {
		e.Run(t, cmdbase...)
		addr1, err := address.StringToUint160("Nhfg3TbpwogLvDGVvAvqyThbsHgoSUKwtn")
		require.NoError(t, err)
		e.checkNextLine(t, "^Account "+address.Uint160ToString(addr1))
		e.checkNextLine(t, "^\\s*GAS:\\s+GasToken \\("+e.Chain.UtilityTokenHash().StringLE()+"\\)")
		balance := e.Chain.GetUtilityTokenBalance(addr1)
		e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+fixedn.Fixed8(balance.Int64()).String()+"$")
		e.checkNextLine(t, "^\\s*Updated:")
		e.checkNextLine(t, "^\\s*$")

		addr2, err := address.StringToUint160("NgEisvCqr2h8wpRxQb7bVPWUZdbVCY8Uo6")
		require.NoError(t, err)
		e.checkNextLine(t, "^Account "+address.Uint160ToString(addr2))
		e.checkNextLine(t, "^\\s*$")

		addr3, err := address.StringToUint160("NNudMSGzEoktFzdYGYoNb3bzHzbmM1genF")
		require.NoError(t, err)
		e.checkNextLine(t, "^Account "+address.Uint160ToString(addr3))
		// The order of assets is undefined.
		for i := 0; i < 2; i++ {
			line := e.getNextLine(t)
			if strings.Contains(line, "GAS") {
				e.checkLine(t, line, "^\\s*GAS:\\s+GasToken \\("+e.Chain.UtilityTokenHash().StringLE()+"\\)")
				balance = e.Chain.GetUtilityTokenBalance(addr3)
				e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+fixedn.Fixed8(balance.Int64()).String()+"$")
				e.checkNextLine(t, "^\\s*Updated:")
			} else {
				balance, index := e.Chain.GetGoverningTokenBalance(validatorHash)
				e.checkLine(t, line, "^\\s*NEO:\\s+NeoToken \\("+e.Chain.GoverningTokenHash().StringLE()+"\\)")
				e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+balance.String()+"$")
				e.checkNextLine(t, "^\\s*Updated\\s*:\\s*"+strconv.FormatUint(uint64(index), 10))
			}
		}

		e.checkNextLine(t, "^\\s*$")
		addr4, err := address.StringToUint160("NaZjSxmRZ4ErG2QEXCQMjjJfvAxMPiutmi") // deployed verify.go contract
		require.NoError(t, err)
		e.checkNextLine(t, "^Account "+address.Uint160ToString(addr4))
		e.checkEOF(t)
	})
	t.Run("Bad token", func(t *testing.T) {
		e.Run(t, append(cmd, "--token", "kek")...)
		e.checkNextLine(t, "^\\s*Account\\s+"+validatorAddr)
		e.checkEOF(t)
	})
	t.Run("Bad wallet", func(t *testing.T) {
		e.RunWithError(t, append(cmdbalance, "--wallet", "/dev/null")...)
	})
	return
}

func TestNEP17Transfer(t *testing.T) {
	w, err := wallet.NewWalletFromFile("testdata/testwallet.json")
	require.NoError(t, err)
	defer w.Close()

	e := newExecutor(t, true)
	args := []string{
		"neo-go", "wallet", "nep17", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", validatorWallet,
		"--to", w.Accounts[0].Address,
		"--token", "NEO",
		"--amount", "1",
		"--from", validatorAddr,
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

	hVerify := deployVerifyContract(t, e)
	const validatorDefault = "Nhfg3TbpwogLvDGVvAvqyThbsHgoSUKwtn"

	t.Run("default address", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "nep17", "multitransfer",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet,
			"--from", validatorAddr,
			"NEO:"+validatorDefault+":42",
			"GAS:"+validatorDefault+":7")
		e.checkTxPersisted(t)

		args := args[:len(args)-2] // cut '--from' argument
		e.In.WriteString("one\r")
		e.Run(t, args...)
		e.checkTxPersisted(t)

		sh, err := address.StringToUint160(w.Accounts[0].Address)
		require.NoError(t, err)
		b, _ := e.Chain.GetGoverningTokenBalance(sh)
		require.Equal(t, big.NewInt(2), b)

		sh, err = address.StringToUint160(validatorDefault)
		require.NoError(t, err)
		b, _ = e.Chain.GetGoverningTokenBalance(sh)
		require.Equal(t, big.NewInt(41), b)
	})

	t.Run("with signers", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "nep17", "multitransfer",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet,
			"--from", validatorAddr,
			"NEO:"+validatorDefault+":42",
			"GAS:"+validatorDefault+":7",
			"--", validatorAddr+":Global")
		e.checkTxPersisted(t)
	})

	validTil := e.Chain.BlockHeight() + 100
	cmd := []string{
		"neo-go", "wallet", "nep17", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", validatorWallet,
		"--to", address.Uint160ToString(e.Chain.GetNotaryContractScriptHash()),
		"--token", "GAS",
		"--amount", "1",
		"--from", validatorAddr,
		"[", validatorAddr, strconv.Itoa(int(validTil)), "]"}

	t.Run("with data", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, cmd...)
		e.checkTxPersisted(t)
	})

	t.Run("with data and signers", func(t *testing.T) {
		t.Run("invalid sender's scope", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.RunWithError(t, append(cmd, "--", validatorAddr+":None")...)
		})
		t.Run("good", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.Run(t, append(cmd, "--", validatorAddr+":Global")...) // CalledByEntry is enough, but it's the default value, so check something else
			e.checkTxPersisted(t)
		})
		t.Run("several signers", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.Run(t, append(cmd, "--", validatorAddr, hVerify.StringLE())...)
			e.checkTxPersisted(t)
		})
	})
}

func TestNEP17MultiTransfer(t *testing.T) {
	privs, _ := generateKeys(t, 3)

	e := newExecutor(t, true)
	neoContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.Neo)
	require.NoError(t, err)
	args := []string{
		"neo-go", "wallet", "nep17", "multitransfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", validatorWallet,
		"--from", validatorAddr,
		"NEO:" + privs[0].Address() + ":42",
		"GAS:" + privs[1].Address() + ":7",
		neoContractHash.StringLE() + ":" + privs[2].Address() + ":13",
	}
	hVerify := deployVerifyContract(t, e)

	t.Run("no cosigners", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, args...)
		e.checkTxPersisted(t)

		b, _ := e.Chain.GetGoverningTokenBalance(privs[0].GetScriptHash())
		require.Equal(t, big.NewInt(42), b)
		b = e.Chain.GetUtilityTokenBalance(privs[1].GetScriptHash())
		require.Equal(t, big.NewInt(int64(fixedn.Fixed8FromInt64(7))), b)
		b, _ = e.Chain.GetGoverningTokenBalance(privs[2].GetScriptHash())
		require.Equal(t, big.NewInt(13), b)
	})

	t.Run("invalid sender scope", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.RunWithError(t, append(args,
			"--", validatorAddr+":None")...) // invalid sender scope
	})
	t.Run("Global sender scope", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, append(args,
			"--", validatorAddr+":Global")...)
		e.checkTxPersisted(t)
	})
	t.Run("Several cosigners", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, append(args,
			"--", validatorAddr, hVerify.StringLE())...)
		e.checkTxPersisted(t)
	})
}

func TestNEP17ImportToken(t *testing.T) {
	e := newExecutor(t, true)

	tmpDir := os.TempDir()
	walletPath := path.Join(tmpDir, "walletForImport.json")
	defer os.Remove(walletPath)

	neoContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.Neo)
	require.NoError(t, err)
	gasContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.Gas)
	require.NoError(t, err)
	nnsContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.NameService)
	require.NoError(t, err)
	e.Run(t, "neo-go", "wallet", "init", "--wallet", walletPath)

	// missing token hash
	e.RunWithError(t, "neo-go", "wallet", "nep17", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", walletPath)

	e.Run(t, "neo-go", "wallet", "nep17", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", walletPath,
		"--token", gasContractHash.StringLE())
	e.Run(t, "neo-go", "wallet", "nep17", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", walletPath,
		"--token", address.Uint160ToString(neoContractHash)) // try address instead of sh

	// not a NEP17 token
	e.RunWithError(t, "neo-go", "wallet", "nep17", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", walletPath,
		"--token", nnsContractHash.StringLE())

	t.Run("Info", func(t *testing.T) {
		checkGASInfo := func(t *testing.T) {
			e.checkNextLine(t, "^Name:\\s*GasToken")
			e.checkNextLine(t, "^Symbol:\\s*GAS")
			e.checkNextLine(t, "^Hash:\\s*"+gasContractHash.StringLE())
			e.checkNextLine(t, "^Decimals:\\s*8")
			e.checkNextLine(t, "^Address:\\s*"+address.Uint160ToString(gasContractHash))
			e.checkNextLine(t, "^Standard:\\s*"+string(manifest.NEP17StandardName))
		}
		t.Run("WithToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep17", "info",
				"--wallet", walletPath, "--token", gasContractHash.StringLE())
			checkGASInfo(t)
		})
		t.Run("NoToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep17", "info",
				"--wallet", walletPath)
			checkGASInfo(t)
			_, err := e.Out.ReadString('\n')
			require.NoError(t, err)
			e.checkNextLine(t, "^Name:\\s*NeoToken")
			e.checkNextLine(t, "^Symbol:\\s*NEO")
			e.checkNextLine(t, "^Hash:\\s*"+neoContractHash.StringLE())
			e.checkNextLine(t, "^Decimals:\\s*0")
			e.checkNextLine(t, "^Address:\\s*"+address.Uint160ToString(neoContractHash))
			e.checkNextLine(t, "^Standard:\\s*"+string(manifest.NEP17StandardName))
		})
		t.Run("Remove", func(t *testing.T) {
			e.In.WriteString("y\r")
			e.Run(t, "neo-go", "wallet", "nep17", "remove",
				"--wallet", walletPath, "--token", neoContractHash.StringLE())
			e.Run(t, "neo-go", "wallet", "nep17", "info",
				"--wallet", walletPath)
			checkGASInfo(t)
			_, err := e.Out.ReadString('\n')
			require.Equal(t, err, io.EOF)
		})
	})
}
