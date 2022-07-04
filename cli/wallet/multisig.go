package wallet

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/urfave/cli"
)

func signStoredTransaction(ctx *cli.Context) error {
	wall, pass, err := readWallet(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	c, err := paramcontext.Read(ctx.String("in"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}
	acc, err := getDecryptedAccount(wall, addrFlag.Uint160(), pass)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx, ok := c.Verifiable.(*transaction.Transaction)
	if !ok {
		return cli.NewExitError("verifiable item is not a transaction", 1)
	}

	ch, err := address.StringToUint160(acc.Address)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("wallet contains invalid account: %s", acc.Address), 1)
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

	priv := acc.PrivateKey()
	sign := priv.SignHashable(uint32(c.Network), tx)
	if err := c.AddSignature(ch, acc.Contract, priv.PublicKey(), sign); err != nil {
		return cli.NewExitError(fmt.Errorf("can't add signature: %w", err), 1)
	}
	if out := ctx.String("out"); out != "" {
		if err := paramcontext.Save(c, out); err != nil {
			return cli.NewExitError(fmt.Errorf("failed to dump resulting transaction: %w", err), 1)
		}
	}
	if len(ctx.String(options.RPCEndpointFlag)) != 0 {
		for i := range tx.Signers {
			w, err := c.GetWitness(tx.Signers[i].Account)
			if err != nil {
				return cli.NewExitError(fmt.Errorf("failed to construct witness for signer #%d: %w", i, err), 1)
			}
			tx.Scripts = append(tx.Scripts, *w)
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
