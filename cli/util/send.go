package util

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/waiter"
	"github.com/urfave/cli/v2"
)

func sendTx(ctx *cli.Context) error {
	args := ctx.Args().Slice()
	if len(args) == 0 {
		return cli.Exit("missing input file", 1)
	} else if len(args) > 1 {
		return cli.Exit("only one input file is accepted", 1)
	}

	pc, err := paramcontext.Read(args[0])
	if err != nil {
		return cli.Exit(err, 1)
	}

	tx, err := pc.GetCompleteTransaction()
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to complete transaction: %w", err), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to create RPC client: %w", err), 1)
	}
	res, err := c.SendRawTransaction(tx)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to submit transaction to RPC node: %w", err), 1)
	}
	var aer *state.AppExecResult
	if ctx.Bool("await") {
		version, err := c.GetVersion()
		aer, err = waiter.New(c, version).Wait(res, tx.ValidUntilBlock, err)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to await transaction %s: %w", res.StringLE(), err), 1)
		}
	}
	txctx.DumpTransactionInfo(ctx.App.Writer, res, aer)
	return nil
}
