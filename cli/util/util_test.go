package util_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestUtilConvert(t *testing.T) {
	e := testcli.NewExecutor(t, false)

	e.Run(t, "neo-go", "util", "convert", util.Uint160{1, 2, 3}.StringLE())
	e.CheckNextLine(t, "f975")                                                                             // int to hex
	e.CheckNextLine(t, "\\+XU=")                                                                           // int to base64
	e.CheckNextLine(t, "NKuyBkoGdZZSLyPbJEetheRhMrGSCQx7YL")                                               // BE to address
	e.CheckNextLine(t, "NL1JGiyJXdTkvFksXbFxgLJcWLj8Ewe7HW")                                               // LE to address
	e.CheckNextLine(t, "Hex to String")                                                                    // hex to string
	e.CheckNextLine(t, "5753853598078696051256155186041784866529345536")                                   // hex to int
	e.CheckNextLine(t, "0102030000000000000000000000000000000000")                                         // swap endianness
	e.CheckNextLine(t, "Base64 to String")                                                                 // base64 to string
	e.CheckNextLine(t, "368753434210909009569191652203865891677393101439813372294890211308228051")         // base64 to bigint
	e.CheckNextLine(t, "30303030303030303030303030303030303030303030303030303030303030303030303330323031") // string to hex
	e.CheckNextLine(t, "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAzMDIwMQ==")                         // string to base64
	e.CheckEOF(t)
}

func TestUtilOps(t *testing.T) {
	e := testcli.NewExecutor(t, false)
	base64Str := "EUA="
	hexStr := "1140"

	check := func(t *testing.T) {
		e.CheckNextLine(t, "INDEX.*OPCODE.*PARAMETER")
		e.CheckNextLine(t, "PUSH1")
		e.CheckNextLine(t, "RET")
		e.CheckEOF(t)
	}

	e.Run(t, "neo-go", "util", "ops", base64Str) // base64
	check(t)

	e.Run(t, "neo-go", "util", "ops", hexStr) // base64 is checked firstly by default, but it's invalid script if decoded from base64
	e.CheckNextLine(t, "INDEX.*OPCODE.*PARAMETER")
	e.CheckNextLine(t, ".*ERROR: incorrect opcode")
	e.CheckEOF(t)

	e.Run(t, "neo-go", "util", "ops", "--hex", hexStr) // explicitly specify hex encoding
	check(t)

	e.RunWithError(t, "neo-go", "util", "ops", "%&~*") // unknown encoding

	tmp := filepath.Join(t.TempDir(), "script_base64.txt")
	require.NoError(t, os.WriteFile(tmp, []byte(base64Str), os.ModePerm))
	e.Run(t, "neo-go", "util", "ops", "--in", tmp) // base64 from file
	check(t)

	tmp = filepath.Join(t.TempDir(), "script_hex.txt")
	require.NoError(t, os.WriteFile(tmp, []byte(hexStr), os.ModePerm))
	e.Run(t, "neo-go", "util", "ops", "--hex", "--in", tmp) // hex from file
	check(t)
}

func TestUtilCancelTx(t *testing.T) {
	e := testcli.NewExecutorSuspended(t)

	w, err := wallet.NewWalletFromFile("../testdata/testwallet.json")
	require.NoError(t, err)

	transferArgs := []string{
		"neo-go", "wallet", "nep17", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--to", w.Accounts[0].Address,
		"--token", "NEO",
		"--from", testcli.ValidatorAddr,
		"--force",
	}
	args := []string{"neo-go", "util", "canceltx",
		"-r", "http://" + e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--address", testcli.ValidatorAddr}

	e.In.WriteString("one\r")
	e.Run(t, append(transferArgs, "--amount", "1")...)
	line := e.GetNextLine(t)
	txHash, err := util.Uint256DecodeStringLE(line)
	require.NoError(t, err)

	_, ok := e.Chain.GetMemPool().TryGetValue(txHash)
	require.True(t, ok)

	t.Run("invalid", func(t *testing.T) {
		t.Run("missing tx argument", func(t *testing.T) {
			e.RunWithError(t, args...)
		})
		t.Run("excessive arguments", func(t *testing.T) {
			e.RunWithError(t, append(args, txHash.StringLE(), txHash.StringLE())...)
		})
		t.Run("invalid hash", func(t *testing.T) {
			e.RunWithError(t, append(args, "notahash")...)
		})
		t.Run("not signed by main signer", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.RunWithError(t, "neo-go", "util", "canceltx",
				"-r", "http://"+e.RPC.Addresses()[0],
				"--wallet", testcli.ValidatorWallet,
				"--address", testcli.MultisigAddr, txHash.StringLE())
		})
		t.Run("wrong rpc endpoint", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.RunWithError(t, "neo-go", "util", "canceltx",
				"-r", "http://localhost:20331",
				"--wallet", testcli.ValidatorWallet, txHash.StringLE())
		})
	})

	e.In.WriteString("one\r")
	e.Run(t, append(args, txHash.StringLE())...)
	resHash, err := util.Uint256DecodeStringLE(e.GetNextLine(t))
	require.NoError(t, err)

	_, _, err = e.Chain.GetTransaction(resHash)
	require.NoError(t, err)
	e.CheckEOF(t)
	go e.Chain.Run()

	require.Eventually(t, func() bool {
		_, aerErr := e.Chain.GetAppExecResults(resHash, trigger.Application)
		return aerErr == nil
	}, time.Second*2, time.Millisecond*50)
}

func TestAwaitUtilCancelTx(t *testing.T) {
	e := testcli.NewExecutor(t, true)

	w, err := wallet.NewWalletFromFile("../testdata/testwallet.json")
	require.NoError(t, err)

	transferArgs := []string{
		"neo-go", "wallet", "nep17", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--to", w.Accounts[0].Address,
		"--token", "NEO",
		"--from", testcli.ValidatorAddr,
		"--force",
	}
	args := []string{"neo-go", "util", "canceltx",
		"-r", "http://" + e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--address", testcli.ValidatorAddr,
		"--await"}

	e.In.WriteString("one\r")
	e.Run(t, append(transferArgs, "--amount", "1")...)
	line := e.GetNextLine(t)
	txHash, err := util.Uint256DecodeStringLE(line)
	require.NoError(t, err)

	_, ok := e.Chain.GetMemPool().TryGetValue(txHash)
	require.True(t, ok)

	e.In.WriteString("one\r")
	e.Run(t, append(args, txHash.StringLE())...)

	response := e.GetNextLine(t)
	if strings.Contains(response, "Conflicting transaction accepted") {
		resHash, _ := e.CheckAwaitableTxPersisted(t)
		require.Eventually(t, func() bool {
			_, aerErr := e.Chain.GetAppExecResults(resHash.Hash(), trigger.Application)
			return aerErr == nil
		}, time.Second*2, time.Millisecond*50)
	} else if strings.Contains(response, "Target transaction accepted") {
		require.Eventually(t, func() bool {
			_, _, err := e.Chain.GetTransaction(txHash)
			require.NoError(t, err, "original transaction should be on chain")
			_, aerErr := e.Chain.GetAppExecResults(txHash, trigger.Application)
			return aerErr == nil
		}, time.Second*2, time.Millisecond*50)
	} else {
		t.Fatalf("unexpected response: %s", response)
	}
}
