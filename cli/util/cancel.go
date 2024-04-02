package util

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/waiter"
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

	acc, w, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to get account from context to sign the conflicting transaction: %w", err), 1)
	}
	defer w.Close()

	signers, err := cmdargs.GetSignersAccounts(acc, w, nil, transaction.CalledByEntry)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid signers: %w", err), 1)
	}
	c, a, exitErr := options.GetRPCWithActor(gctx, ctx, signers)
	if exitErr != nil {
		return exitErr
	}

	mainTx, _ := c.GetRawTransactionVerbose(txHash)
	if mainTx != nil && !mainTx.Blockhash.Equals(util.Uint256{}) {
		return cli.NewExitError(fmt.Errorf("target transaction %s is accepted at block %s", txHash, mainTx.Blockhash.StringLE()), 1)
	}

	if mainTx != nil && !mainTx.HasSigner(acc.ScriptHash()) {
		return cli.NewExitError(fmt.Errorf("account %s is not a signer of the conflicting transaction", acc.Address), 1)
	}

	resHash, resVub, err := a.SendTunedRun([]byte{byte(opcode.RET)}, []transaction.Attribute{{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: txHash}}}, func(r *result.Invoke, t *transaction.Transaction) error {
		err := actor.DefaultCheckerModifier(r, t)
		if err != nil {
			return err
		}
		if mainTx != nil && t.NetworkFee < mainTx.NetworkFee+1 {
			t.NetworkFee = mainTx.NetworkFee + 1
		}
		t.NetworkFee += int64(flags.Fixed8FromContext(ctx, "gas"))
		if mainTx != nil {
			t.ValidUntilBlock = mainTx.ValidUntilBlock
		}
		return nil
	})
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to send conflicting transaction: %w", err), 1)
	}
	var res *state.AppExecResult
	if ctx.Bool("await") {
		res, err = a.WaitAny(gctx, resVub, txHash, resHash)
		if err != nil {
			if errors.Is(err, waiter.ErrTxNotAccepted) {
				if mainTx == nil {
					return cli.NewExitError(fmt.Errorf("neither target nor conflicting transaction is accepted before the current height %d (ValidUntilBlock value of conlicting transaction). Main transaction is unknown to the provided RPC node, thus still has chances to be accepted, you may try cancellation again", resVub), 1)
				}
				fmt.Fprintf(ctx.App.Writer, "Neither target nor conflicting transaction is accepted before the current height %d (ValidUntilBlock value of both target and conflicting transactions). Main transaction is not valid anymore, cancellation is successful\n", resVub)
				return nil
			}
			return cli.NewExitError(fmt.Errorf("failed to await target/ conflicting transaction %s/ %s: %w", txHash.StringLE(), resHash.StringLE(), err), 1)
		}
		if txHash.Equals(res.Container) {
			tx, err := c.GetRawTransactionVerbose(txHash)
			if err != nil {
				return cli.NewExitError(fmt.Errorf("target transaction %s is accepted", txHash), 1)
			}
			return cli.NewExitError(fmt.Errorf("target transaction %s is accepted at block %s", txHash, tx.Blockhash.StringLE()), 1)
		}
		fmt.Fprintln(ctx.App.Writer, "Conflicting transaction accepted")
	}
	txctx.DumpTransactionInfo(ctx.App.Writer, resHash, res)
	return nil
}
