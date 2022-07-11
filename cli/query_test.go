package main

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/random"
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
	e := newExecutorSuspended(t)

	w, err := wallet.NewWalletFromFile("testdata/testwallet.json")
	require.NoError(t, err)

	transferArgs := []string{
		"neo-go", "wallet", "nep17", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", validatorWallet,
		"--to", w.Accounts[0].Address,
		"--token", "NEO",
		"--from", validatorAddr,
		"--force",
	}

	e.In.WriteString("one\r")
	e.Run(t, append(transferArgs, "--amount", "1")...)
	line := e.getNextLine(t)
	txHash, err := util.Uint256DecodeStringLE(line)
	require.NoError(t, err)

	tx, ok := e.Chain.GetMemPool().TryGetValue(txHash)
	require.True(t, ok)

	args := []string{"neo-go", "query", "tx", "--rpc-endpoint", "http://" + e.RPC.Addr}
	e.Run(t, append(args, txHash.StringLE())...)
	e.checkNextLine(t, `Hash:\s+`+txHash.StringLE())
	e.checkNextLine(t, `OnChain:\s+false`)
	e.checkNextLine(t, `ValidUntil:\s+`+strconv.FormatUint(uint64(tx.ValidUntilBlock), 10))
	e.checkEOF(t)

	height := e.Chain.BlockHeight()
	go e.Chain.Run()
	require.Eventually(t, func() bool { return e.Chain.BlockHeight() > height }, time.Second*2, time.Millisecond*50)

	e.Run(t, append(args, txHash.StringLE())...)
	e.checkNextLine(t, `Hash:\s+`+txHash.StringLE())
	e.checkNextLine(t, `OnChain:\s+true`)

	_, height, err = e.Chain.GetTransaction(txHash)
	require.NoError(t, err)
	e.checkNextLine(t, `BlockHash:\s+`+e.Chain.GetHeaderHash(int(height)).StringLE())
	e.checkNextLine(t, `Success:\s+true`)
	e.checkEOF(t)

	t.Run("verbose", func(t *testing.T) {
		e.Run(t, append(args, "--verbose", txHash.StringLE())...)
		e.compareQueryTxVerbose(t, tx)

		t.Run("FAULT", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.Run(t, "neo-go", "contract", "invokefunction",
				"--rpc-endpoint", "http://"+e.RPC.Addr,
				"--wallet", validatorWallet,
				"--address", validatorAddr,
				"--force",
				random.Uint160().StringLE(),
				"randomMethod")

			e.checkNextLine(t, `Warning:`)
			e.checkNextLine(t, "Sending transaction")
			line := strings.TrimPrefix(e.getNextLine(t), "Sent invocation transaction ")
			txHash, err := util.Uint256DecodeStringLE(line)
			require.NoError(t, err)

			height := e.Chain.BlockHeight()
			require.Eventually(t, func() bool { return e.Chain.BlockHeight() > height }, time.Second*2, time.Millisecond*50)

			tx, _, err := e.Chain.GetTransaction(txHash)
			require.NoError(t, err)
			e.Run(t, append(args, "--verbose", txHash.StringLE())...)
			e.compareQueryTxVerbose(t, tx)
		})
	})

	t.Run("invalid", func(t *testing.T) {
		t.Run("missing tx argument", func(t *testing.T) {
			e.RunWithError(t, args...)
		})
		t.Run("invalid hash", func(t *testing.T) {
			e.RunWithError(t, append(args, "notahash")...)
		})
		t.Run("good hash, missing tx", func(t *testing.T) {
			e.RunWithError(t, append(args, random.Uint256().StringLE())...)
		})
	})
}

func (e *executor) compareQueryTxVerbose(t *testing.T, tx *transaction.Transaction) {
	e.checkNextLine(t, `Hash:\s+`+tx.Hash().StringLE())
	e.checkNextLine(t, `OnChain:\s+true`)
	_, height, err := e.Chain.GetTransaction(tx.Hash())
	require.NoError(t, err)
	e.checkNextLine(t, `BlockHash:\s+`+e.Chain.GetHeaderHash(int(height)).StringLE())

	res, _ := e.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
	e.checkNextLine(t, fmt.Sprintf(`Success:\s+%t`, res[0].Execution.VMState == vmstate.Halt))
	for _, s := range tx.Signers {
		e.checkNextLine(t, fmt.Sprintf(`Signer:\s+%s\s*\(%s\)`, address.Uint160ToString(s.Account), s.Scopes.String()))
	}
	e.checkNextLine(t, `SystemFee:\s+`+fixedn.Fixed8(tx.SystemFee).String()+" GAS$")
	e.checkNextLine(t, `NetworkFee:\s+`+fixedn.Fixed8(tx.NetworkFee).String()+" GAS$")
	e.checkNextLine(t, `Script:\s+`+regexp.QuoteMeta(base64.StdEncoding.EncodeToString(tx.Script)))
	c := vm.NewContext(tx.Script)
	n := 0
	for ; c.NextIP() < c.LenInstr(); _, _, err = c.Next() {
		require.NoError(t, err)
		n++
	}
	e.checkScriptDump(t, n)

	if res[0].Execution.VMState != vmstate.Halt {
		e.checkNextLine(t, `Exception:\s+`+regexp.QuoteMeta(res[0].Execution.FaultException))
	}
	e.checkEOF(t)
}

func TestQueryHeight(t *testing.T) {
	e := newExecutor(t, true)

	e.Run(t, "neo-go", "query", "height", "--rpc-endpoint", "http://"+e.RPC.Addr)
	e.checkNextLine(t, `^Latest block: [0-9]+$`)
	e.checkNextLine(t, `^Validated state: [0-9]+$`)
	e.checkEOF(t)
}
