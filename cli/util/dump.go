package util

import (
	"encoding/json"
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/cli/query"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/urfave/cli"
)

func txDump(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return cli.NewExitError("missing input file", 1)
	} else if len(args) > 1 {
		return cli.NewExitError("only one input file is accepted", 1)
	}

	c, err := paramcontext.Read(args[0])
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx, ok := c.Verifiable.(*transaction.Transaction)
	if !ok {
		return cli.NewExitError("verifiable item is not a transaction", 1)
	}

	query.DumpApplicationLog(ctx, nil, tx, nil, true)

	if ctx.String(options.RPCEndpointFlag) != "" {
		gctx, cancel := options.GetTimeoutContext(ctx)
		defer cancel()

		var err error
		cl, err := options.GetRPCClient(gctx, ctx)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		res, err := cl.InvokeScript(tx.Script, tx.Signers)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		resS, err := json.MarshalIndent(res, "", " ")
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		fmt.Fprintln(ctx.App.Writer, string(resS))
	}
	return nil
}
