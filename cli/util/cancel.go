package util

import (
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/urfave/cli"
)

func cancelTx(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return cli.NewExitError("transaction hash is missing", 1)
	} else if len(args) > 1 {
		return cli.NewExitError("only one transaction hash is accepted", 1)
	}

	txHash, err := util.Uint256DecodeStringLE(strings.TrimPrefix(args[0], "0x"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("invalid tx hash: %s", args[0]), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create RPC client: %w", err), 1)
	}

	mainTx, _ := c.GetRawTransactionVerbose(txHash)
	if mainTx != nil && !mainTx.Blockhash.Equals(util.Uint256{}) {
		return cli.NewExitError(fmt.Errorf("transaction %s is already accepted at block %s", txHash, mainTx.Blockhash.StringLE()), 1)
	}
	acc, w, err := smartcontract.GetAccFromContext(ctx)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to get account from context to sign the conflicting transaction: %w", err), 1)
	}
	defer w.Close()

	if mainTx != nil && !mainTx.HasSigner(acc.ScriptHash()) {
		return cli.NewExitError(fmt.Errorf("account %s is not a signer of the conflicting transaction", acc.Address), 1)
	}

	a, err := actor.NewSimple(c, acc)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create Actor: %w", err), 1)
	}

	resHash, _, err := a.SendTunedRun([]byte{byte(opcode.RET)}, []transaction.Attribute{{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: txHash}}}, func(r *result.Invoke, t *transaction.Transaction) error {
		err := actor.DefaultCheckerModifier(r, t)
		if err != nil {
			return err
		}
		if mainTx != nil && t.NetworkFee < mainTx.NetworkFee+1 {
			t.NetworkFee = mainTx.NetworkFee + 1
		}
		t.NetworkFee += int64(flags.Fixed8FromContext(ctx, "gas"))
		return nil
	})
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to send conflicting transaction: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, resHash.StringLE())
	return nil
}
