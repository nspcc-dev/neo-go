package wallet

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
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
