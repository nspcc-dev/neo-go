package wallet

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

func newNEP11Commands() []cli.Command {
	tokenAddressFlag := flags.AddressFlag{
		Name:  "token",
		Usage: "Token contract address or hash in LE",
	}
	ownerAddressFlag := flags.AddressFlag{
		Name:  "address",
		Usage: "NFT owner address or hash in LE",
	}
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
			UsageText: "transfer --wallet <path> --rpc-endpoint <node> --timeout <time> --from <addr> --to <addr> --token <hash-or-name> --id <token-id> [--amount string] [data] [-- <cosigner1:Scope> [<cosigner2> [...]]]",
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
		{
			Name:      "properties",
			Usage:     "print properties of NEP11 token",
			UsageText: "properties --rpc-endpoint <node> --timeout <time> --token <hash> --id <token-id>",
			Action:    printNEP11Properties,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				tokenID,
			}, options.RPC...),
		},
		{
			Name:      "ownerOf",
			Usage:     "print owner of non-divisible NEP11 token with the specified ID",
			UsageText: "ownerOf --rpc-endpoint <node> --timeout <time> --token <hash> --id <token-id>",
			Action:    printNEP11Owner,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				tokenID,
			}, options.RPC...),
		},
		{
			Name:      "tokensOf",
			Usage:     "print list of tokens IDs for the specified NFT owner",
			UsageText: "tokensOf --rpc-endpoint <node> --timeout <time> --token <hash> --address <addr>",
			Action:    printNEP11TokensOf,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				ownerAddressFlag,
			}, options.RPC...),
		},
		{
			Name:      "tokens",
			Usage:     "print list of tokens IDs minted by the specified NFT (optional method)",
			UsageText: "tokens --rpc-endpoint <node> --timeout <time> --token <hash>",
			Action:    printNEP11Tokens,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
			}, options.RPC...),
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
		tokenHash, err := flags.ParseAddress(name)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't fetch matching token from RPC-node: %w", err), 1)
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

func signAndSendNEP11Transfer(ctx *cli.Context, c *client.Client, acc *wallet.Account, token, to util.Uint160, tokenID string, amount *big.Int, data interface{}, cosigners []client.SignerAccount) error {
	gas := flags.Fixed8FromContext(ctx, "gas")
	sysgas := flags.Fixed8FromContext(ctx, "sysgas")

	var (
		tx  *transaction.Transaction
		err error
	)
	if amount != nil {
		var from util.Uint160

		from, err = address.StringToUint160(acc.Address)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("bad account address: %w", err), 1)
		}
		tx, err = c.CreateNEP11TransferTx(acc, token, int64(gas), cosigners, from, to, amount, tokenID, data)
	} else {
		tx, err = c.CreateNEP11TransferTx(acc, token, int64(gas), cosigners, to, tokenID, data)
	}
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	tx.SystemFee += int64(sysgas)

	if outFile := ctx.String("out"); outFile != "" {
		if err := paramcontext.InitAndSave(c.GetNetwork(), tx, acc, outFile); err != nil {
			return cli.NewExitError(err, 1)
		}
	} else {
		if !ctx.Bool("force") {
			err := input.ConfirmTx(ctx.App.Writer, tx)
			if err != nil {
				return cli.NewExitError(err, 1)
			}
		}
		_, err := c.SignAndPushTx(tx, acc, cosigners)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	fmt.Fprintln(ctx.App.Writer, tx.Hash().StringLE())
	return nil
}

func printNEP11Owner(ctx *cli.Context) error {
	var err error
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.NewExitError("token contract hash was not set", 1)
	}

	tokenID := ctx.String("id")
	if tokenID == "" {
		return cli.NewExitError(errors.New("token ID should be specified"), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	result, err := c.NEP11NDOwnerOf(tokenHash.Uint160(), tokenID)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("failed to call NEP11 `ownerOf` method: %s", err.Error()), 1)
	}

	fmt.Fprintln(ctx.App.Writer, address.Uint160ToString(result))
	return nil
}

func printNEP11TokensOf(ctx *cli.Context) error {
	var err error
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.NewExitError("token contract hash was not set", 1)
	}

	acc := ctx.Generic("address").(*flags.Address)
	if !acc.IsSet {
		return cli.NewExitError("owner address flag was not set", 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	result, err := c.NEP11TokensOf(tokenHash.Uint160(), acc.Uint160())
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("failed to call NEP11 `tokensOf` method: %s", err.Error()), 1)
	}

	for i := range result {
		fmt.Fprintln(ctx.App.Writer, result[i])
	}
	return nil
}

func printNEP11Tokens(ctx *cli.Context) error {
	var err error
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.NewExitError("token contract hash was not set", 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	result, err := c.NEP11Tokens(tokenHash.Uint160())
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("failed to call optional NEP11 `tokens` method: %s", err.Error()), 1)
	}

	for i := range result {
		fmt.Fprintln(ctx.App.Writer, result[i])
	}
	return nil
}

func printNEP11Properties(ctx *cli.Context) error {
	var err error
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.NewExitError("token contract hash was not set", 1)
	}

	tokenID := ctx.String("id")
	if tokenID == "" {
		return cli.NewExitError(errors.New("token ID should be specified"), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	result, err := c.NEP11Properties(tokenHash.Uint160(), tokenID)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("failed to call NEP11 `properties` method: %s", err.Error()), 1)
	}

	bytes, err := stackitem.ToJSON(result)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("failed to convert result to JSON: %s", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, string(bytes))
	return nil
}
