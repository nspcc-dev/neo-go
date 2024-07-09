package wallet

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli/v2"
)

func newNEP11Commands() []*cli.Command {
	maxIters := strconv.Itoa(config.DefaultMaxIteratorResultItems)
	tokenAddressFlag := &flags.AddressFlag{
		Name:  "token",
		Usage: "Token contract address or hash in LE",
	}
	ownerAddressFlag := &flags.AddressFlag{
		Name:  "address",
		Usage: "NFT owner address or hash in LE",
	}
	tokenID := &cli.StringFlag{
		Name:  "id",
		Usage: "Hex-encoded token ID",
	}

	balanceFlags := make([]cli.Flag, len(baseBalanceFlags))
	copy(balanceFlags, baseBalanceFlags)
	balanceFlags = append(balanceFlags, tokenID)
	balanceFlags = append(balanceFlags, options.RPC...)
	transferFlags := make([]cli.Flag, len(baseTransferFlags))
	copy(transferFlags, baseTransferFlags)
	transferFlags = append(transferFlags, tokenID)
	transferFlags = append(transferFlags, options.RPC...)
	return []*cli.Command{
		{
			Name:      "balance",
			Usage:     "Get address balance",
			UsageText: "balance -w wallet [--wallet-config path] --rpc-endpoint <node> [--timeout <time>] [--address <address>] [--token <hash-or-name>] [--id <token-id>]",
			Description: `Prints NEP-11 balances for address and assets/IDs specified. By default (no
   address or token parameter) all tokens (NFT contracts) for all accounts in
   the specified wallet are listed with all tokens (actual NFTs) insied. A
   single account can be chosen with the address option and/or a single NFT
   contract can be selected with the token option. Further, you can specify a
   particular NFT ID (hex-encoded) to display (which is mostly useful for
   divisible NFTs). Tokens can be specified by hash, address, name or symbol.
   Hashes and addresses always work (as long as they belong to a correct NEP-11
   contract), while names or symbols are matched against the token data
   stored in the wallet (see import command) or balance data returned from the
   server. If the token is not specified directly (with hash/address) and is
   not found in the wallet then depending on the balances data from the server
   this command can print no data at all or print multiple tokens for one
   account (if they use the same names/symbols).
`,
			Action: getNEP11Balance,
			Flags:  balanceFlags,
		},
		{
			Name:      "import",
			Usage:     "Import NEP-11 token to a wallet",
			UsageText: "import -w wallet [--wallet-config path] --rpc-endpoint <node> --timeout <time> --token <hash>",
			Action:    importNEP11Token,
			Flags:     importFlags,
		},
		{
			Name:      "info",
			Usage:     "Print imported NEP-11 token info",
			UsageText: "print -w wallet [--wallet-config path] [--token <hash-or-name>]",
			Action:    printNEP11Info,
			Flags: []cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				tokenFlag,
			},
		},
		{
			Name:      "remove",
			Usage:     "Remove NEP-11 token from the wallet",
			UsageText: "remove -w wallet [--wallet-config path] --token <hash-or-name>",
			Action:    removeNEP11Token,
			Flags: []cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				tokenFlag,
				txctx.ForceFlag,
			},
		},
		{
			Name:      "transfer",
			Usage:     "Transfer NEP-11 tokens",
			UsageText: "transfer -w wallet [--wallet-config path] --rpc-endpoint <node> --timeout <time> --from <addr> --to <addr> --token <hash-or-name> --id <token-id> [--amount string] [--await] [data] [-- <cosigner1:Scope> [<cosigner2> [...]]]",
			Action:    transferNEP11,
			Flags:     transferFlags,
			Description: `Transfers specified NEP-11 token with optional cosigners list attached to
   the transfer. Amount should be specified for divisible NEP-11
   tokens and omitted for non-divisible NEP-11 tokens. See
   'contract testinvokefunction' documentation for the details
   about cosigners syntax. If no cosigners are given then the
   sender with CalledByEntry scope will be used as the only
   signer. If --await flag is set then the command will wait
   for the transaction to be included in a block.
`,
		},
		{
			Name:      "properties",
			Usage:     "Print properties of NEP-11 token",
			UsageText: "properties --rpc-endpoint <node> [--timeout <time>] --token <hash> --id <token-id> [--historic <block/hash>]",
			Action:    printNEP11Properties,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				tokenID,
				options.Historic,
			}, options.RPC...),
		},
		{
			Name:      "ownerOf",
			Usage:     "Print owner of non-divisible NEP-11 token with the specified ID",
			UsageText: "ownerOf --rpc-endpoint <node> [--timeout <time>] --token <hash> --id <token-id> [--historic <block/hash>]",
			Action:    printNEP11NDOwner,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				tokenID,
				options.Historic,
			}, options.RPC...),
		},
		{
			Name:      "ownerOfD",
			Usage:     "Print set of owners of divisible NEP-11 token with the specified ID (" + maxIters + " will be printed at max)",
			UsageText: "ownerOfD --rpc-endpoint <node> [--timeout <time>] --token <hash> --id <token-id> [--historic <block/hash>]",
			Action:    printNEP11DOwner,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				tokenID,
				options.Historic,
			}, options.RPC...),
		},
		{
			Name:      "tokensOf",
			Usage:     "Print list of tokens IDs for the specified NFT owner (" + maxIters + " will be printed at max)",
			UsageText: "tokensOf --rpc-endpoint <node> [--timeout <time>] --token <hash> --address <addr> [--historic <block/hash>]",
			Action:    printNEP11TokensOf,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				ownerAddressFlag,
				options.Historic,
			}, options.RPC...),
		},
		{
			Name:      "tokens",
			Usage:     "Print list of tokens IDs minted by the specified NFT (optional method; " + maxIters + " will be printed at max)",
			UsageText: "tokens --rpc-endpoint <node> [--timeout <time>] --token <hash> [--historic <block/hash>]",
			Action:    printNEP11Tokens,
			Flags: append([]cli.Flag{
				tokenAddressFlag,
				options.Historic,
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
	return getNEPBalance(ctx, manifest.NEP11StandardName, func(ctx *cli.Context, c *rpcclient.Client, addrHash util.Uint160, name string, token *wallet.Token, nftID string) error {
		balances, err := c.GetNEP11Balances(addrHash)
		if err != nil {
			return err
		}
		var tokenFound bool
		for i := range balances.Balances {
			curToken := tokenFromNEP11Balance(&balances.Balances[i])
			if tokenMatch(curToken, token, name) {
				printNFTBalance(ctx, balances.Balances[i], nftID)
				tokenFound = true
			}
		}
		if name == "" || tokenFound {
			return nil
		}
		if token != nil {
			// We have an exact token, but there is no balance data for it -> print without NFTs.
			printNFTBalance(ctx, result.NEP11AssetBalance{
				Asset:    token.Hash,
				Decimals: int(token.Decimals),
				Name:     token.Name,
				Symbol:   token.Symbol,
			}, "")
		} else {
			// We have no data for this token at all, maybe it's not even correct -> complain.
			fmt.Fprintf(ctx.App.Writer, "Can't find data for %q token\n", name)
		}
		return nil
	})
}

func printNFTBalance(ctx *cli.Context, balance result.NEP11AssetBalance, nftID string) {
	fmt.Fprintf(ctx.App.Writer, "%s: %s (%s)\n", balance.Symbol, balance.Name, balance.Asset.StringLE())
	for _, tok := range balance.Tokens {
		if len(nftID) > 0 && nftID != tok.ID {
			continue
		}
		fmt.Fprintf(ctx.App.Writer, "\tToken: %s\n", tok.ID)
		fmt.Fprintf(ctx.App.Writer, "\t\tAmount: %s\n", decimalAmount(tok.Amount, balance.Decimals))
		fmt.Fprintf(ctx.App.Writer, "\t\tUpdated: %d\n", tok.LastUpdated)
	}
}

func transferNEP11(ctx *cli.Context) error {
	return transferNEP(ctx, manifest.NEP11StandardName)
}

func printNEP11NDOwner(ctx *cli.Context) error {
	return printNEP11Owner(ctx, false)
}

func printNEP11DOwner(ctx *cli.Context) error {
	return printNEP11Owner(ctx, true)
}

func printNEP11Owner(ctx *cli.Context, divisible bool) error {
	var err error
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.Exit("token contract hash was not set", 1)
	}

	tokenID := ctx.String("id")
	if tokenID == "" {
		return cli.Exit(errors.New("token ID should be specified"), 1)
	}
	tokenIDBytes, err := hex.DecodeString(tokenID)
	if err != nil {
		return cli.Exit(fmt.Errorf("invalid tokenID bytes: %w", err), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	_, inv, err := options.GetRPCWithInvoker(gctx, ctx, nil)
	if err != nil {
		return err
	}

	if divisible {
		n11 := nep11.NewDivisibleReader(inv, tokenHash.Uint160())
		result, err := n11.OwnerOfExpanded(tokenIDBytes, config.DefaultMaxIteratorResultItems)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to call NEP-11 divisible `ownerOf` method: %s", err.Error()), 1)
		}
		for _, h := range result {
			fmt.Fprintln(ctx.App.Writer, address.Uint160ToString(h))
		}
	} else {
		n11 := nep11.NewNonDivisibleReader(inv, tokenHash.Uint160())
		result, err := n11.OwnerOf(tokenIDBytes)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to call NEP-11 non-divisible `ownerOf` method: %s", err.Error()), 1)
		}
		fmt.Fprintln(ctx.App.Writer, address.Uint160ToString(result))
	}

	return nil
}

func printNEP11TokensOf(ctx *cli.Context) error {
	var err error
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.Exit("token contract hash was not set", 1)
	}

	acc := ctx.Generic("address").(*flags.Address)
	if !acc.IsSet {
		return cli.Exit("owner address flag was not set", 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	_, inv, err := options.GetRPCWithInvoker(gctx, ctx, nil)
	if err != nil {
		return err
	}

	n11 := nep11.NewBaseReader(inv, tokenHash.Uint160())
	result, err := n11.TokensOfExpanded(acc.Uint160(), config.DefaultMaxIteratorResultItems)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to call NEP-11 `tokensOf` method: %s", err.Error()), 1)
	}

	for i := range result {
		fmt.Fprintln(ctx.App.Writer, hex.EncodeToString(result[i]))
	}
	return nil
}

func printNEP11Tokens(ctx *cli.Context) error {
	var err error
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.Exit("token contract hash was not set", 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	_, inv, err := options.GetRPCWithInvoker(gctx, ctx, nil)
	if err != nil {
		return err
	}

	n11 := nep11.NewBaseReader(inv, tokenHash.Uint160())
	result, err := n11.TokensExpanded(config.DefaultMaxIteratorResultItems)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to call optional NEP-11 `tokens` method: %s", err.Error()), 1)
	}

	for i := range result {
		fmt.Fprintln(ctx.App.Writer, hex.EncodeToString(result[i]))
	}
	return nil
}

func printNEP11Properties(ctx *cli.Context) error {
	var err error
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	tokenHash := ctx.Generic("token").(*flags.Address)
	if !tokenHash.IsSet {
		return cli.Exit("token contract hash was not set", 1)
	}

	tokenID := ctx.String("id")
	if tokenID == "" {
		return cli.Exit(errors.New("token ID should be specified"), 1)
	}
	tokenIDBytes, err := hex.DecodeString(tokenID)
	if err != nil {
		return cli.Exit(fmt.Errorf("invalid tokenID bytes: %w", err), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	_, inv, err := options.GetRPCWithInvoker(gctx, ctx, nil)
	if err != nil {
		return err
	}

	n11 := nep11.NewBaseReader(inv, tokenHash.Uint160())
	result, err := n11.Properties(tokenIDBytes)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to call NEP-11 `properties` method: %s", err.Error()), 1)
	}

	bytes, err := stackitem.ToJSON(result)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to convert result to JSON: %s", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, string(bytes))
	return nil
}
