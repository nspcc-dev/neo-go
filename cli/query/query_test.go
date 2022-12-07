package query_test

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestQueryTx(t *testing.T) {
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

	e.In.WriteString("one\r")
	e.Run(t, append(transferArgs, "--amount", "1")...)
	line := e.GetNextLine(t)
	txHash, err := util.Uint256DecodeStringLE(line)
	require.NoError(t, err)

	tx, ok := e.Chain.GetMemPool().TryGetValue(txHash)
	require.True(t, ok)

	args := []string{"neo-go", "query", "tx", "--rpc-endpoint", "http://" + e.RPC.Addresses()[0]}
	e.Run(t, append(args, txHash.StringLE())...)
	e.CheckNextLine(t, `Hash:\s+`+txHash.StringLE())
	e.CheckNextLine(t, `OnChain:\s+false`)
	e.CheckNextLine(t, `ValidUntil:\s+`+strconv.FormatUint(uint64(tx.ValidUntilBlock), 10))
	e.CheckEOF(t)

	go e.Chain.Run()
	require.Eventually(t, func() bool { _, aerErr := e.Chain.GetAppExecResults(txHash, trigger.Application); return aerErr == nil }, time.Second*2, time.Millisecond*50)

	e.Run(t, append(args, txHash.StringLE())...)
	e.CheckNextLine(t, `Hash:\s+`+txHash.StringLE())
	e.CheckNextLine(t, `OnChain:\s+true`)

	_, height, err := e.Chain.GetTransaction(txHash)
	require.NoError(t, err)
	e.CheckNextLine(t, `BlockHash:\s+`+e.Chain.GetHeaderHash(height).StringLE())
	e.CheckNextLine(t, `Success:\s+true`)
	e.CheckEOF(t)

	t.Run("verbose", func(t *testing.T) {
		e.Run(t, append(args, "--verbose", txHash.StringLE())...)
		compareQueryTxVerbose(t, e, tx)

		t.Run("FAULT", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.Run(t, "neo-go", "contract", "invokefunction",
				"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
				"--wallet", testcli.ValidatorWallet,
				"--address", testcli.ValidatorAddr,
				"--force",
				random.Uint160().StringLE(),
				"randomMethod")

			e.CheckNextLine(t, `Warning:`)
			e.CheckNextLine(t, "Sending transaction")
			line := strings.TrimPrefix(e.GetNextLine(t), "Sent invocation transaction ")
			txHash, err := util.Uint256DecodeStringLE(line)
			require.NoError(t, err)

			require.Eventually(t, func() bool { _, aerErr := e.Chain.GetAppExecResults(txHash, trigger.Application); return aerErr == nil }, time.Second*2, time.Millisecond*50)

			tx, _, err := e.Chain.GetTransaction(txHash)
			require.NoError(t, err)
			e.Run(t, append(args, "--verbose", txHash.StringLE())...)
			compareQueryTxVerbose(t, e, tx)
		})
	})

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
		t.Run("good hash, missing tx", func(t *testing.T) {
			e.RunWithError(t, append(args, random.Uint256().StringLE())...)
		})
	})
}

func compareQueryTxVerbose(t *testing.T, e *testcli.Executor, tx *transaction.Transaction) {
	e.CheckNextLine(t, `Hash:\s+`+tx.Hash().StringLE())
	e.CheckNextLine(t, `OnChain:\s+true`)
	_, height, err := e.Chain.GetTransaction(tx.Hash())
	require.NoError(t, err)
	e.CheckNextLine(t, `BlockHash:\s+`+e.Chain.GetHeaderHash(height).StringLE())

	res, _ := e.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
	e.CheckNextLine(t, fmt.Sprintf(`Success:\s+%t`, res[0].Execution.VMState == vmstate.Halt))
	for _, s := range tx.Signers {
		e.CheckNextLine(t, fmt.Sprintf(`Signer:\s+%s\s*\(%s\)`, address.Uint160ToString(s.Account), s.Scopes.String()))
	}
	e.CheckNextLine(t, `SystemFee:\s+`+fixedn.Fixed8(tx.SystemFee).String()+" GAS$")
	e.CheckNextLine(t, `NetworkFee:\s+`+fixedn.Fixed8(tx.NetworkFee).String()+" GAS$")
	e.CheckNextLine(t, `Script:\s+`+regexp.QuoteMeta(base64.StdEncoding.EncodeToString(tx.Script)))
	c := vm.NewContext(tx.Script)
	n := 0
	for ; c.NextIP() < c.LenInstr(); _, _, err = c.Next() {
		require.NoError(t, err)
		n++
	}
	e.CheckScriptDump(t, n)

	if res[0].Execution.VMState != vmstate.Halt {
		e.CheckNextLine(t, `Exception:\s+`+regexp.QuoteMeta(res[0].Execution.FaultException))
	}
	e.CheckEOF(t)
}

func TestQueryHeight(t *testing.T) {
	e := testcli.NewExecutor(t, true)

	args := []string{"neo-go", "query", "height", "--rpc-endpoint", "http://" + e.RPC.Addresses()[0]}
	e.Run(t, args...)
	e.CheckNextLine(t, `^Latest block: [0-9]+$`)
	e.CheckNextLine(t, `^Validated state: [0-9]+$`)
	e.CheckEOF(t)
	t.Run("excessive arguments", func(t *testing.T) {
		e.RunWithError(t, append(args, "something")...)
	})
}
