package wallet

import (
	"encoding/json"
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/urfave/cli"
)

func signStoredTransaction(ctx *cli.Context) error {
	var (
		out      = ctx.String("out")
		rpcNode  = ctx.String(options.RPCEndpointFlag)
		addrFlag = ctx.Generic("address").(*flags.Address)
	)
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, pass, err := readWallet(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	pc, err := paramcontext.Read(ctx.String("in"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}

	var ch = addrFlag.Uint160()
	acc, err := getDecryptedAccount(wall, ch, pass)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx, ok := pc.Verifiable.(*transaction.Transaction)
	if !ok {
		return cli.NewExitError("verifiable item is not a transaction", 1)
	}

	signerFound := false
	for i := range tx.Signers {
		if tx.Signers[i].Account == ch {
			signerFound = true
			break
		}
	}
	if !signerFound {
		return cli.NewExitError("tx signers don't contain provided account", 1)
	}

	if acc.CanSign() {
		priv := acc.PrivateKey()
		sign := priv.SignHashable(uint32(pc.Network), pc.Verifiable)
		if err := pc.AddSignature(ch, acc.Contract, priv.PublicKey(), sign); err != nil {
			return cli.NewExitError(fmt.Errorf("can't add signature: %w", err), 1)
		}
	} else if rpcNode == "" {
		return cli.NewExitError(fmt.Errorf("can't sign transactions with the given account and no RPC endpoing given to send anything signed"), 1)
	}
	// Not saving and not sending, print.
	if out == "" && rpcNode == "" {
		txt, err := json.MarshalIndent(pc, " ", "     ")
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't display resulting context: %w", err), 1)
		}
		fmt.Fprintln(ctx.App.Writer, string(txt))
		return nil
	}
	if out != "" {
		if err := paramcontext.Save(pc, out); err != nil {
			return cli.NewExitError(fmt.Errorf("can't save resulting context: %w", err), 1)
		}
	}
	if rpcNode != "" {
		tx, err = pc.GetCompleteTransaction()
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to complete transaction: %w", err), 1)
		}

		gctx, cancel := options.GetTimeoutContext(ctx)
		defer cancel()

		var err error // `GetRPCClient` returns specialized type.
		c, err := options.GetRPCClient(gctx, ctx)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to create RPC client: %w", err), 1)
		}
		res, err := c.SendRawTransaction(tx)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to submit transaction to RPC node: %w", err), 1)
		}
		fmt.Fprintln(ctx.App.Writer, res.StringLE())
		return nil
	}

	fmt.Fprintln(ctx.App.Writer, tx.Hash().StringLE())
	return nil
}
