package wallet

import (
	"encoding/json"
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/waiter"
	"github.com/urfave/cli/v2"
)

func signStoredTransaction(ctx *cli.Context) error {
	var (
		out      = ctx.String("out")
		rpcNode  = ctx.String(options.RPCEndpointFlag)
		addrFlag = ctx.Generic("address").(*flags.Address)
		aer      *state.AppExecResult
	)
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}

	pc, err := paramcontext.Read(ctx.String("in"))
	if err != nil {
		return cli.Exit(err, 1)
	}

	if !addrFlag.IsSet {
		return cli.Exit("address was not provided", 1)
	}
	acc, _, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}

	tx, ok := pc.Verifiable.(*transaction.Transaction)
	if !ok {
		return cli.Exit("verifiable item is not a transaction", 1)
	}

	if !tx.HasSigner(acc.ScriptHash()) {
		return cli.Exit("tx signers don't contain provided account", 1)
	}

	if acc.CanSign() {
		sign := acc.SignHashable(pc.Network, pc.Verifiable)
		if err := pc.AddSignature(acc.ScriptHash(), acc.Contract, acc.PublicKey(), sign); err != nil {
			return cli.Exit(fmt.Errorf("can't add signature: %w", err), 1)
		}
	} else if rpcNode == "" {
		return cli.Exit(fmt.Errorf("can't sign transactions with the given account and no RPC endpoing given to send anything signed"), 1)
	}
	// Not saving and not sending, print.
	if out == "" && rpcNode == "" {
		txt, err := json.MarshalIndent(pc, " ", "     ")
		if err != nil {
			return cli.Exit(fmt.Errorf("can't display resulting context: %w", err), 1)
		}
		fmt.Fprintln(ctx.App.Writer, string(txt))
		return nil
	}
	if out != "" {
		if err := paramcontext.Save(pc, out); err != nil {
			return cli.Exit(fmt.Errorf("can't save resulting context: %w", err), 1)
		}
	}
	if rpcNode != "" {
		tx, err = pc.GetCompleteTransaction()
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to complete transaction: %w", err), 1)
		}

		gctx, cancel := options.GetTimeoutContext(ctx)
		defer cancel()

		var err error // `GetRPCClient` returns specialized type.
		c, err := options.GetRPCClient(gctx, ctx)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to create RPC client: %w", err), 1)
		}
		res, err := c.SendRawTransaction(tx)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to submit transaction to RPC node: %w", err), 1)
		}
		if ctx.Bool("await") {
			version, err := c.GetVersion()
			aer, err = waiter.New(c, version).Wait(res, tx.ValidUntilBlock, err)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to await transaction %s: %w", res.StringLE(), err), 1)
			}
		}
	}

	txctx.DumpTransactionInfo(ctx.App.Writer, tx.Hash(), aer)
	return nil
}
