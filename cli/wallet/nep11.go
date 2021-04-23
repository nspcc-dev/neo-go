package wallet

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

func newNEP11Commands() []cli.Command {
	tokenID := cli.StringFlag{
		Name:  "id",
		Usage: "Token ID",
	}

	balanceFlags := make([]cli.Flag, len(baseBalanceFlags))
	copy(balanceFlags, baseBalanceFlags)
	balanceFlags = append(balanceFlags, tokenID)
	balanceFlags = append(balanceFlags, options.RPC...)
	transferFlags := make([]cli.Flag, len(baseTransferFlags))
	copy(transferFlags, baseTransferFlags)
	transferFlags = append(transferFlags, tokenID)
	transferFlags = append(transferFlags, options.RPC...)
	return []cli.Command{
		{
			Name:      "balance",
			Usage:     "get address balance",
			UsageText: "balance --wallet <path> --rpc-endpoint <node> [--timeout <time>] [--address <address>] --token <hash-or-name> [--id <token-id>]",
			Action:    getNEP11Balance,
			Flags:     balanceFlags,
		},
		{
			Name:      "import",
			Usage:     "import NEP11 token to a wallet",
			UsageText: "import --wallet <path> --rpc-endpoint <node> --timeout <time> --token <hash>",
			Action:    importNEP11Token,
			Flags:     importFlags,
		},
		{
			Name:      "info",
			Usage:     "print imported NEP11 token info",
			UsageText: "print --wallet <path> [--token <hash-or-name>]",
			Action:    printNEP11Info,
			Flags: []cli.Flag{
				walletPathFlag,
				tokenFlag,
			},
		},
		{
			Name:      "remove",
			Usage:     "remove NEP11 token from the wallet",
			UsageText: "remove --wallet <path> --token <hash-or-name>",
			Action:    removeNEP11Token,
			Flags: []cli.Flag{
				walletPathFlag,
				tokenFlag,
				forceFlag,
			},
		},
		{
			Name:      "transfer",
			Usage:     "transfer NEP11 tokens",
			UsageText: "transfer --wallet <path> --rpc-endpoint <node> --timeout <time> --from <addr> --to <addr> --token <hash-or-name> --id <token-id> [--amount string] [-- <cosigner1:Scope> [<cosigner2> [...]]]",
			Action:    transferNEP11,
			Flags:     transferFlags,
			Description: `Transfers specified NEP11 token with optional cosigners list attached to 
   the transfer. Amount should be specified for divisible NEP11
   tokens and omitted for non-divisible NEP11 tokens. See
   'contract testinvokefunction' documentation for the details
   about cosigners syntax. If no cosigners are given then the
   sender with CalledByEntry scope will be used as the only
   signer.
`,
		},
	}
}

func importNEP11Token(ctx *cli.Context) error {
	return importNEPToken(ctx, manifest.NEP11StandardName)
}

func printNEP11Info(ctx *cli.Context) error {
	return printNEPInfo(ctx, manifest.NEP11StandardName)
}

func removeNEP11Token(ctx *cli.Context) error {
	return removeNEPToken(ctx, manifest.NEP11StandardName)
}

func getNEP11Balance(ctx *cli.Context) error {
	var accounts []*wallet.Account

	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("bad wallet: %w", err), 1)
	}
	defer wall.Close()

	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		addrHash := addrFlag.Uint160()
		acc := wall.GetAccount(addrHash)
		if acc == nil {
			return cli.NewExitError(fmt.Errorf("can't find account for the address: %s", address.Uint160ToString(addrHash)), 1)
		}
		accounts = append(accounts, acc)
	} else {
		if len(wall.Accounts) == 0 {
			return cli.NewExitError(errors.New("no accounts in the wallet"), 1)
		}
		accounts = wall.Accounts
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	name := ctx.String("token")
	if name == "" {
		return cli.NewExitError("token hash or name should be specified", 1)
	}
	token, err := getMatchingToken(ctx, wall, name, manifest.NEP11StandardName)
	if err != nil {
		fmt.Fprintln(ctx.App.ErrWriter, "Can't find matching token in the wallet. Querying RPC-node for token info.")
		tokenHash, err := flags.ParseAddress(name)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("valid token adress or hash in LE should be specified for RPC-node request: %s", err.Error()), 1)
		}
		token, err = c.NEP11TokenInfo(tokenHash)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	tokenID := ctx.String("id")
	for k, acc := range accounts {
		addrHash, err := address.StringToUint160(acc.Address)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid account address: %w", err), 1)
		}

		if k != 0 {
			fmt.Fprintln(ctx.App.Writer)
		}
		fmt.Fprintf(ctx.App.Writer, "Account %s\n", acc.Address)

		var amount int64
		if tokenID == "" {
			amount, err = c.NEP11BalanceOf(token.Hash, addrHash)
		} else {
			amount, err = c.NEP11DBalanceOf(token.Hash, addrHash, tokenID)
		}
		if err != nil {
			continue
		}
		amountStr := fixedn.ToString(big.NewInt(amount), int(token.Decimals))

		format := "%s: %s (%s)\n"
		formatArgs := []interface{}{token.Symbol, token.Name, token.Hash.StringLE()}
		if tokenID != "" {
			format = "%s: %s (%s, %s)\n"
			formatArgs = append(formatArgs, tokenID)
		}
		fmt.Fprintf(ctx.App.Writer, format, formatArgs...)
		fmt.Fprintf(ctx.App.Writer, "\tAmount : %s\n", amountStr)

	}
	return nil
}

func transferNEP11(ctx *cli.Context) error {
	return transferNEP(ctx, manifest.NEP11StandardName)
}

func signAndSendNEP11Transfer(ctx *cli.Context, c *client.Client, acc *wallet.Account, token, to util.Uint160, tokenID string, amount *big.Int, cosigners []client.SignerAccount) error {
	gas := flags.Fixed8FromContext(ctx, "gas")

	var (
		tx  *transaction.Transaction
		err error
	)
	if amount != nil {
		from, err := address.StringToUint160(acc.Address)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("bad account address: %w", err), 1)
		}
		tx, err = c.CreateNEP11TransferTx(acc, token, int64(gas), cosigners, from, to, amount, tokenID)
	} else {
		tx, err = c.CreateNEP11TransferTx(acc, token, int64(gas), cosigners, to, tokenID)
	}
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if outFile := ctx.String("out"); outFile != "" {
		if err := paramcontext.InitAndSave(c.GetNetwork(), tx, acc, outFile); err != nil {
			return cli.NewExitError(err, 1)
		}
	} else {
		_, err := c.SignAndPushTx(tx, acc, cosigners)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	fmt.Fprintln(ctx.App.Writer, tx.Hash().StringLE())
	return nil
}
