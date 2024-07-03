/*
Package txctx contains helper functions that deal with transactions in CLI context.
*/
package txctx

import (
	"fmt"
	"io"
	"time"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

var (
	// GasFlag is a flag used for the additional network fee.
	GasFlag = flags.Fixed8Flag{
		Name:  "gas, g",
		Usage: "Network fee to add to the transaction (prioritizing it)",
	}
	// SysGasFlag is a flag used for the additional system fee.
	SysGasFlag = flags.Fixed8Flag{
		Name:  "sysgas, e",
		Usage: "System fee to add to the transaction (compensating for execution)",
	}
	// OutFlag is a flag used for file output.
	OutFlag = cli.StringFlag{
		Name:  "out",
		Usage: "File (JSON) to put signature context with a transaction to",
	}
	// ForceFlag is a flag used to force transaction send.
	ForceFlag = cli.BoolFlag{
		Name:  "force",
		Usage: "Do not ask for a confirmation (and ignore errors)",
	}
	// AwaitFlag is a flag used to wait for the transaction to be included in a block.
	AwaitFlag = cli.BoolFlag{
		Name:  "await",
		Usage: "Wait for the transaction to be included in a block",
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
		aer    *state.AppExecResult
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
			tx.ValidUntilBlock += uint32(waitTime.Milliseconds()/int64(ver.Protocol.MillisecondsPerBlock)) + 2
		}
		var (
			resTx util.Uint256
			vub   uint32
		)
		resTx, vub, err = act.SignAndSend(tx)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		if ctx.Bool("await") {
			aer, err = act.Wait(resTx, vub, err)
			if err != nil {
				return cli.NewExitError(fmt.Errorf("failed to await transaction %s: %w", resTx.StringLE(), err), 1)
			}
		}
	}
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	DumpTransactionInfo(ctx.App.Writer, tx.Hash(), aer)
	return nil
}

// DumpTransactionInfo prints transaction info to the given writer.
func DumpTransactionInfo(w io.Writer, h util.Uint256, res *state.AppExecResult) {
	fmt.Fprintln(w, h.StringLE())
	if res != nil {
		fmt.Fprintf(w, "OnChain:\t%t\n", res != nil)
		fmt.Fprintf(w, "VMState:\t%s\n", res.VMState.String())
		if res.FaultException != "" {
			fmt.Fprintf(w, "FaultException:\t%s\n", res.FaultException)
		}
	}
}
