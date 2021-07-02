package wallet

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

var (
	tokenFlag = cli.StringFlag{
		Name:  "token",
		Usage: "Token to use (hash or name (for NEO/GAS or imported tokens))",
	}
	gasFlag = flags.Fixed8Flag{
		Name:  "gas, g",
		Usage: "network fee to add to the transaction (prioritizing it)",
	}
	sysGasFlag = flags.Fixed8Flag{
		Name:  "sysgas, e",
		Usage: "system fee to add to transaction (compensating for execution)",
	}
	baseBalanceFlags = []cli.Flag{
		walletPathFlag,
		tokenFlag,
		flags.AddressFlag{
			Name:  "address, a",
			Usage: "Address to use",
		},
	}
	importFlags = append([]cli.Flag{
		walletPathFlag,
		flags.AddressFlag{
			Name:  "token",
			Usage: "Token contract address or hash in LE",
		},
	}, options.RPC...)
	baseTransferFlags = []cli.Flag{
		walletPathFlag,
		outFlag,
		fromAddrFlag,
		toAddrFlag,
		tokenFlag,
		gasFlag,
		sysGasFlag,
		cli.StringFlag{
			Name:  "amount",
			Usage: "Amount of asset to send",
		},
	}
	multiTransferFlags = append([]cli.Flag{
		walletPathFlag,
		outFlag,
		fromAddrFlag,
		gasFlag,
		sysGasFlag,
	}, options.RPC...)
)

func newNEP17Commands() []cli.Command {
	balanceFlags := make([]cli.Flag, len(baseBalanceFlags))
	copy(balanceFlags, baseBalanceFlags)
	balanceFlags = append(balanceFlags, options.RPC...)
	transferFlags := make([]cli.Flag, len(baseTransferFlags))
	copy(transferFlags, baseTransferFlags)
	transferFlags = append(transferFlags, options.RPC...)
	return []cli.Command{
		{
			Name:      "balance",
			Usage:     "get address balance",
			UsageText: "balance --wallet <path> --rpc-endpoint <node> [--timeout <time>] [--address <address>] [--token <hash-or-name>]",
			Action:    getNEP17Balance,
			Flags:     balanceFlags,
		},
		{
			Name:      "import",
			Usage:     "import NEP17 token to a wallet",
			UsageText: "import --wallet <path> --rpc-endpoint <node> --timeout <time> --token <hash>",
			Action:    importNEP17Token,
			Flags:     importFlags,
		},
		{
			Name:      "info",
			Usage:     "print imported NEP17 token info",
			UsageText: "print --wallet <path> [--token <hash-or-name>]",
			Action:    printNEP17Info,
			Flags: []cli.Flag{
				walletPathFlag,
				tokenFlag,
			},
		},
		{
			Name:      "remove",
			Usage:     "remove NEP17 token from the wallet",
			UsageText: "remove --wallet <path> --token <hash-or-name>",
			Action:    removeNEP17Token,
			Flags: []cli.Flag{
				walletPathFlag,
				tokenFlag,
				forceFlag,
			},
		},
		{
			Name:      "transfer",
			Usage:     "transfer NEP17 tokens",
			UsageText: "transfer --wallet <path> --rpc-endpoint <node> --timeout <time> --from <addr> --to <addr> --token <hash-or-name> --amount string [data] [-- <cosigner1:Scope> [<cosigner2> [...]]]",
			Action:    transferNEP17,
			Flags:     transferFlags,
			Description: `Transfers specified NEP17 token amount with optional 'data' parameter and cosigners
   list attached to the transfer. See 'contract testinvokefunction' documentation
   for the details about 'data' parameter and cosigners syntax. If no 'data' is
   given then default nil value will be used. If no cosigners are given then the
   sender with CalledByEntry scope will be used as the only signer.
`,
		},
		{
			Name:  "multitransfer",
			Usage: "transfer NEP17 tokens to multiple recipients",
			UsageText: `multitransfer --wallet <path> --rpc-endpoint <node> --timeout <time> --from <addr>` +
				` <token1>:<addr1>:<amount1> [<token2>:<addr2>:<amount2> [...]] [-- <cosigner1:Scope> [<cosigner2> [...]]]`,
			Action: multiTransferNEP17,
			Flags:  multiTransferFlags,
		},
	}
}

func getNEP17Balance(ctx *cli.Context) error {
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

	for k, acc := range accounts {
		addrHash, err := address.StringToUint160(acc.Address)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid account address: %w", err), 1)
		}
		balances, err := c.GetNEP17Balances(addrHash)
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		if k != 0 {
			fmt.Fprintln(ctx.App.Writer)
		}
		fmt.Fprintf(ctx.App.Writer, "Account %s\n", acc.Address)

		for i := range balances.Balances {
			var tokenName, tokenSymbol string
			tokenDecimals := 0
			asset := balances.Balances[i].Asset
			token, err := getMatchingToken(ctx, wall, asset.StringLE(), manifest.NEP17StandardName)
			if err != nil {
				token, err = c.NEP17TokenInfo(asset)
			}
			if err == nil {
				if name != "" && !(token.Name == name || token.Symbol == name || token.Address() == name || token.Hash.StringLE() == name) {
					continue
				}
				tokenName = token.Name
				tokenSymbol = token.Symbol
				tokenDecimals = int(token.Decimals)
			} else {
				if name != "" {
					continue
				}
				tokenSymbol = "UNKNOWN"
			}
			fmt.Fprintf(ctx.App.Writer, "%s: %s (%s)\n", tokenSymbol, tokenName, asset.StringLE())
			amount := balances.Balances[i].Amount
			if tokenDecimals != 0 {
				b, ok := new(big.Int).SetString(amount, 10)
				if ok {
					amount = fixedn.ToString(b, tokenDecimals)
				}
			}
			fmt.Fprintf(ctx.App.Writer, "\tAmount : %s\n", amount)
			fmt.Fprintf(ctx.App.Writer, "\tUpdated: %d\n", balances.Balances[i].LastUpdated)
		}
	}
	return nil
}

func getMatchingToken(ctx *cli.Context, w *wallet.Wallet, name string, standard string) (*wallet.Token, error) {
	return getMatchingTokenAux(ctx, func(i int) *wallet.Token {
		return w.Extra.Tokens[i]
	}, len(w.Extra.Tokens), name, standard)
}

func getMatchingTokenRPC(ctx *cli.Context, c *client.Client, addr util.Uint160, name string, standard string) (*wallet.Token, error) {
	switch standard {
	case manifest.NEP17StandardName:
		bs, err := c.GetNEP17Balances(addr)
		if err != nil {
			return nil, err
		}
		get := func(i int) *wallet.Token {
			t, _ := c.NEP17TokenInfo(bs.Balances[i].Asset)
			return t
		}
		return getMatchingTokenAux(ctx, get, len(bs.Balances), name, standard)
	case manifest.NEP11StandardName:
		tokenHash, err := flags.ParseAddress(name)
		if err != nil {
			return nil, fmt.Errorf("valid token adress or hash in LE should be specified for %s RPC-node request: %s", standard, err.Error())
		}
		get := func(i int) *wallet.Token {
			t, _ := c.NEP11TokenInfo(tokenHash)
			return t
		}
		return getMatchingTokenAux(ctx, get, 1, name, standard)
	default:
		return nil, fmt.Errorf("unsupported %s token", standard)
	}
}

func getMatchingTokenAux(ctx *cli.Context, get func(i int) *wallet.Token, n int, name string, standard string) (*wallet.Token, error) {
	var token *wallet.Token
	var count int
	for i := 0; i < n; i++ {
		t := get(i)
		if t != nil && (t.Hash.StringLE() == name || t.Address() == name || t.Symbol == name || t.Name == name) && t.Standard == standard {
			if count == 1 {
				printTokenInfo(ctx, token)
				printTokenInfo(ctx, t)
				return nil, fmt.Errorf("multiple matching %s tokens found", standard)
			}
			count++
			token = t
		}
	}
	if count == 0 {
		return nil, fmt.Errorf("%s token was not found", standard)
	}
	return token, nil
}

func importNEP17Token(ctx *cli.Context) error {
	return importNEPToken(ctx, manifest.NEP17StandardName)
}

func importNEPToken(ctx *cli.Context, standard string) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	tokenHashFlag := ctx.Generic("token").(*flags.Address)
	if !tokenHashFlag.IsSet {
		return cli.NewExitError("token contract hash was not set", 1)
	}
	tokenHash := tokenHashFlag.Uint160()

	for _, t := range wall.Extra.Tokens {
		if t.Hash.Equals(tokenHash) && t.Standard == standard {
			printTokenInfo(ctx, t)
			return cli.NewExitError(fmt.Errorf("%s token already exists", standard), 1)
		}
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var tok *wallet.Token
	switch standard {
	case manifest.NEP17StandardName:
		tok, err = c.NEP17TokenInfo(tokenHash)
	case manifest.NEP11StandardName:
		tok, err = c.NEP11TokenInfo(tokenHash)
	default:
		return cli.NewExitError(fmt.Sprintf("unsupported token standard: %s", standard), 1)
	}
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't receive token info: %w", err), 1)
	}

	wall.AddToken(tok)
	if err := wall.Save(); err != nil {
		return cli.NewExitError(err, 1)
	}
	printTokenInfo(ctx, tok)
	return nil
}

func printTokenInfo(ctx *cli.Context, tok *wallet.Token) {
	w := ctx.App.Writer
	fmt.Fprintf(w, "Name:\t%s\n", tok.Name)
	fmt.Fprintf(w, "Symbol:\t%s\n", tok.Symbol)
	fmt.Fprintf(w, "Hash:\t%s\n", tok.Hash.StringLE())
	fmt.Fprintf(w, "Decimals: %d\n", tok.Decimals)
	fmt.Fprintf(w, "Address: %s\n", tok.Address())
	fmt.Fprintf(w, "Standard:\t%s\n", tok.Standard)
}

func printNEP17Info(ctx *cli.Context) error {
	return printNEPInfo(ctx, manifest.NEP17StandardName)
}

func printNEPInfo(ctx *cli.Context, standard string) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	if name := ctx.String("token"); name != "" {
		token, err := getMatchingToken(ctx, wall, name, standard)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		printTokenInfo(ctx, token)
		return nil
	}

	var count int
	for _, t := range wall.Extra.Tokens {
		if count > 0 {
			fmt.Fprintln(ctx.App.Writer)
		}
		if t.Standard == standard {
			printTokenInfo(ctx, t)
			count++
		}
	}
	return nil
}

func removeNEP17Token(ctx *cli.Context) error {
	return removeNEPToken(ctx, manifest.NEP17StandardName)
}

func removeNEPToken(ctx *cli.Context, standard string) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	token, err := getMatchingToken(ctx, wall, ctx.String("token"), standard)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if !ctx.Bool("force") {
		if ok := askForConsent(ctx.App.Writer); !ok {
			return nil
		}
	}
	if err := wall.RemoveToken(token.Hash); err != nil {
		return cli.NewExitError(fmt.Errorf("can't remove token: %w", err), 1)
	} else if err := wall.Save(); err != nil {
		return cli.NewExitError(fmt.Errorf("error while saving wallet: %w", err), 1)
	}
	return nil
}

func multiTransferNEP17(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	fromFlag := ctx.Generic("from").(*flags.Address)
	from, err := getDefaultAddress(fromFlag, wall)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	acc, err := getDecryptedAccount(ctx, wall, from)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if ctx.NArg() == 0 {
		return cli.NewExitError("empty recipients list", 1)
	}
	var (
		recipients      []client.TransferTarget
		cosignersOffset = ctx.NArg()
	)
	cache := make(map[string]*wallet.Token)
	for i := 0; i < ctx.NArg(); i++ {
		arg := ctx.Args().Get(i)
		if arg == cmdargs.CosignersSeparator {
			cosignersOffset = i + 1
			break
		}
		ss := strings.SplitN(arg, ":", 3)
		if len(ss) != 3 {
			return cli.NewExitError("send format must be '<token>:<addr>:<amount>", 1)
		}
		token, ok := cache[ss[0]]
		if !ok {
			token, err = getMatchingToken(ctx, wall, ss[0], manifest.NEP17StandardName)
			if err != nil {
				token, err = getMatchingTokenRPC(ctx, c, from, ss[0], manifest.NEP17StandardName)
				if err != nil {
					return cli.NewExitError(fmt.Errorf("can't fetch matching token from RPC-node: %w", err), 1)
				}
			}
		}
		cache[ss[0]] = token
		addr, err := address.StringToUint160(ss[1])
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid address: '%s'", ss[1]), 1)
		}
		amount, err := fixedn.FromString(ss[2], int(token.Decimals))
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid amount: %w", err), 1)
		}
		recipients = append(recipients, client.TransferTarget{
			Token:   token.Hash,
			Address: addr,
			Amount:  amount.Int64(),
			Data:    nil,
		})
	}

	cosigners, extErr := cmdargs.GetSignersFromContext(ctx, cosignersOffset)
	if extErr != nil {
		return extErr
	}
	cosignersAccounts, err := cmdargs.GetSignersAccounts(wall, cosigners)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create NEP17 multitransfer transaction: %w", err), 1)
	}

	return signAndSendNEP17Transfer(ctx, c, acc, recipients, cosignersAccounts)
}

func transferNEP17(ctx *cli.Context) error {
	return transferNEP(ctx, manifest.NEP17StandardName)
}

func transferNEP(ctx *cli.Context, standard string) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	fromFlag := ctx.Generic("from").(*flags.Address)
	from, err := getDefaultAddress(fromFlag, wall)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	acc, err := getDecryptedAccount(ctx, wall, from)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	toFlag := ctx.Generic("to").(*flags.Address)
	to := toFlag.Uint160()
	token, err := getMatchingToken(ctx, wall, ctx.String("token"), standard)
	if err != nil {
		token, err = getMatchingTokenRPC(ctx, c, from, ctx.String("token"), standard)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't fetch matching token from RPC-node: %w", err), 1)
		}
	}

	cosignersOffset, data, extErr := cmdargs.GetDataFromContext(ctx)
	if extErr != nil {
		return extErr
	}

	cosigners, extErr := cmdargs.GetSignersFromContext(ctx, cosignersOffset)
	if extErr != nil {
		return extErr
	}
	cosignersAccounts, err := cmdargs.GetSignersAccounts(wall, cosigners)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create NEP17 transfer transaction: %w", err), 1)
	}

	amountArg := ctx.String("amount")
	switch standard {
	case manifest.NEP17StandardName:
		amount, err := fixedn.FromString(amountArg, int(token.Decimals))
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid amount: %w", err), 1)
		}
		return signAndSendNEP17Transfer(ctx, c, acc, []client.TransferTarget{{
			Token:   token.Hash,
			Address: to,
			Amount:  amount.Int64(),
			Data:    data,
		}}, cosignersAccounts)
	case manifest.NEP11StandardName:
		tokenID := ctx.String("id")
		if tokenID == "" {
			return cli.NewExitError(errors.New("token ID should be specified"), 1)
		}
		if amountArg == "" {
			return signAndSendNEP11Transfer(ctx, c, acc, token.Hash, to, tokenID, nil, data, cosignersAccounts)
		}
		amount, err := fixedn.FromString(amountArg, int(token.Decimals))
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid amount: %w", err), 1)
		}
		return signAndSendNEP11Transfer(ctx, c, acc, token.Hash, to, tokenID, amount, data, cosignersAccounts)
	default:
		return cli.NewExitError(fmt.Errorf("unsupported token standard %s", standard), 1)
	}
}

func signAndSendNEP17Transfer(ctx *cli.Context, c *client.Client, acc *wallet.Account, recipients []client.TransferTarget, cosigners []client.SignerAccount) error {
	gas := flags.Fixed8FromContext(ctx, "gas")
	sysgas := flags.Fixed8FromContext(ctx, "sysgas")

	tx, err := c.CreateNEP17MultiTransferTx(acc, int64(gas), recipients, cosigners)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	tx.SystemFee += int64(sysgas)

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

func getDefaultAddress(fromFlag *flags.Address, w *wallet.Wallet) (util.Uint160, error) {
	if fromFlag.IsSet {
		return fromFlag.Uint160(), nil
	}
	addr := w.GetChangeAddress()
	if addr.Equals(util.Uint160{}) {
		return util.Uint160{}, errors.New("can't get default address")
	}
	return addr, nil
}
