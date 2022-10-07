/*
Package txctx contains helper functions that deal with transactions in CLI context.
*/
package txctx

import (
	"fmt"
	"time"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

var (
	// GasFlag is a flag used for the additional network fee.
	GasFlag = flags.Fixed8Flag{
		Name:  "gas, g",
		Usage: "network fee to add to the transaction (prioritizing it)",
	}
	// SysGasFlag is a flag used for the additional system fee.
	SysGasFlag = flags.Fixed8Flag{
		Name:  "sysgas, e",
		Usage: "system fee to add to the transaction (compensating for execution)",
	}
	// OutFlag is a flag used for file output.
	OutFlag = cli.StringFlag{
		Name:  "out",
		Usage: "file (JSON) to put signature context with a transaction to",
	}
	// ForceFlag is a flag used to force transaction send.
	ForceFlag = cli.BoolFlag{
		Name:  "force",
		Usage: "Do not ask for a confirmation (and ignore errors)",
	}
)

// SignAndSend adds network and system fees to the provided transaction and
// either sends it to the network (with a confirmation or --force flag) or saves
// it into a file (given in the --out flag).
func SignAndSend(ctx *cli.Context, act *actor.Actor, acc *wallet.Account, tx *transaction.Transaction) error {
	var (
		err    error
		gas    = flags.Fixed8FromContext(ctx, "gas")
		sysgas = flags.Fixed8FromContext(ctx, "sysgas")
		ver    = act.GetVersion()
	)

	tx.SystemFee += int64(sysgas)
	tx.NetworkFee += int64(gas)

	if outFile := ctx.String("out"); outFile != "" {
		// Make a long-lived transaction, it's to be signed manually.
		tx.ValidUntilBlock += (ver.Protocol.MaxValidUntilBlockIncrement - uint32(ver.Protocol.ValidatorsCount)) - 2
		err = paramcontext.InitAndSave(ver.Protocol.Network, tx, acc, outFile)
	} else {
		if !ctx.Bool("force") {
			promptTime := time.Now()
			err := input.ConfirmTx(ctx.App.Writer, tx)
			if err != nil {
				return cli.NewExitError(err, 1)
			}
			waitTime := time.Since(promptTime)
			// Compensate for confirmation waiting.
			tx.ValidUntilBlock += uint32((waitTime.Milliseconds() / int64(ver.Protocol.MillisecondsPerBlock))) + 1
		}
		_, _, err = act.SignAndSend(tx)
	}
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Fprintln(ctx.App.Writer, tx.Hash().StringLE())
	return nil
}
