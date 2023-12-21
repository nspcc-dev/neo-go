package wallet

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/gas"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neptoken"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

// transferTarget represents target address, token amount and data for transfer.
type transferTarget struct {
	Token   util.Uint160
	Address util.Uint160
	Amount  int64
	Data    any
}

var (
	tokenFlag = cli.StringFlag{
		Name:  "token",
		Usage: "Token to use (hash or name (for NEO/GAS or imported tokens))",
	}
	baseBalanceFlags = []cli.Flag{
		walletPathFlag,
		walletConfigFlag,
		tokenFlag,
		flags.AddressFlag{
			Name:  "address, a",
			Usage: "Address to use",
		},
	}
	importFlags = append([]cli.Flag{
		walletPathFlag,
		walletConfigFlag,
		flags.AddressFlag{
			Name:  "token",
			Usage: "Token contract address or hash in LE",
		},
	}, options.RPC...)
	baseTransferFlags = []cli.Flag{
		walletPathFlag,
		walletConfigFlag,
		txctx.OutFlag,
		fromAddrFlag,
		toAddrFlag,
		tokenFlag,
		txctx.GasFlag,
		txctx.SysGasFlag,
		txctx.ForceFlag,
		cli.StringFlag{
			Name:  "amount",
			Usage: "Amount of asset to send",
		},
	}
	multiTransferFlags = append([]cli.Flag{
		walletPathFlag,
		walletConfigFlag,
		txctx.OutFlag,
		fromAddrFlag,
		txctx.GasFlag,
		txctx.SysGasFlag,
		txctx.ForceFlag,
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
			UsageText: "balance -w wallet [--wallet-config path] --rpc-endpoint <node> [--timeout <time>] [--address <address>] [--token <hash-or-name>]",
			Description: `Prints NEP-17 balances for address and tokens specified. By default (no
   address or token parameter) all tokens for all accounts in the specified wallet
   are listed. A single account can be chosen with the address option and/or a
   single token can be selected with the token option. Tokens can be specified
   by hash, address, name or symbol. Hashes and addresses always work (as long
   as they belong to a correct NEP-17 contract), while names or symbols (if
   they're not NEO or GAS names/symbols) are matched against the token data
   stored in the wallet (see import command) or balance data returned from the
   server. If the token is not specified directly (with hash/address) and is
   not found in the wallet then depending on the balances data from the server
   this command can print no data at all or print multiple tokens for one
   account (if they use the same names/symbols).
`,
			Action: getNEP17Balance,
			Flags:  flags.MarkRequired(balanceFlags, options.RPCEndpointFlag+", r"),
		},
		{
			Name:      "import",
			Usage:     "import NEP-17 token to a wallet",
			UsageText: "import -w wallet [--wallet-config path] --rpc-endpoint <node> --timeout <time> --token <hash>",
			Action:    importNEP17Token,
			Flags:     flags.MarkRequired(importFlags, options.RPCEndpointFlag+", r"),
		},
		{
			Name:      "info",
			Usage:     "print imported NEP-17 token info",
			UsageText: "print -w wallet [--wallet-config path] [--token <hash-or-name>]",
			Action:    printNEP17Info,
			Flags: []cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				tokenFlag,
			},
		},
		{
			Name:      "remove",
			Usage:     "remove NEP-17 token from the wallet",
			UsageText: "remove -w wallet [--wallet-config path] --token <hash-or-name>",
			Action:    removeNEP17Token,
			Flags: []cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				tokenFlag,
				txctx.ForceFlag,
			},
		},
		{
			Name:      "transfer",
			Usage:     "transfer NEP-17 tokens",
			UsageText: "transfer -w wallet [--wallet-config path] --rpc-endpoint <node> [--timeout <time>] --from <addr> --to <addr> --token <hash-or-name> --amount string [data] [-- <cosigner1:Scope> [<cosigner2> [...]]]",
			Action:    transferNEP17,
			Flags:     flags.MarkRequired(transferFlags, options.RPCEndpointFlag+", r", "from", "to", "amount"),
			Description: `Transfers specified NEP-17 token amount with optional 'data' parameter and cosigners
   list attached to the transfer. See 'contract testinvokefunction' documentation
   for the details about 'data' parameter and cosigners syntax. If no 'data' is
   given then default nil value will be used. If no cosigners are given then the
   sender with CalledByEntry scope will be used as the only signer.
`,
		},
		{
			Name:  "multitransfer",
			Usage: "transfer NEP-17 tokens to multiple recipients",
			UsageText: `multitransfer -w wallet [--wallet-config path] --rpc-endpoint <node> [--timeout <time>] --from <addr>` +
				` <token1>:<addr1>:<amount1> [<token2>:<addr2>:<amount2> [...]] [-- <cosigner1:Scope> [<cosigner2> [...]]]`,
			Action: multiTransferNEP17,
			Flags:  flags.MarkRequired(multiTransferFlags, options.RPCEndpointFlag+", r", "from"),
		},
	}
}

func tokenMatch(curToken *wallet.Token, expToken *wallet.Token, name string) bool {
	return name == "" || // No specification at all, everything matches.
		(expToken != nil && expToken.Hash == curToken.Hash) || // Exact token specification, matches perfectly.
		(expToken == nil && name != "" && (curToken.Name == name || curToken.Symbol == name)) // Loose (named non-native) token specification, best-effort.
}

func getNEP17Balance(ctx *cli.Context) error {
	return getNEPBalance(ctx, manifest.NEP17StandardName, func(ctx *cli.Context, c *rpcclient.Client, addrHash util.Uint160, name string, token *wallet.Token, _ string) error {
		balances, err := c.GetNEP17Balances(addrHash)
		if err != nil {
			return err
		}

		var tokenFound bool
		for i := range balances.Balances {
			curToken := tokenFromNEP17Balance(&balances.Balances[i])
			if tokenMatch(curToken, token, name) {
				printAssetBalance(ctx, balances.Balances[i])
				tokenFound = true
			}
		}
		if name == "" || tokenFound {
			return nil
		}
		if token != nil {
			// We have an exact token, but there is no balance data for it -> print 0.
			printAssetBalance(ctx, result.NEP17Balance{
				Asset:       token.Hash,
				Amount:      "0",
				Decimals:    int(token.Decimals),
				LastUpdated: 0,
				Name:        token.Name,
				Symbol:      token.Symbol,
			})
		} else {
			// We have no data for this token at all, maybe it's not even correct -> complain.
			fmt.Fprintf(ctx.App.Writer, "Can't find data for %q token\n", name)
		}
		return nil
	})
}

func getNEPBalance(ctx *cli.Context, standard string, accHandler func(*cli.Context, *rpcclient.Client, util.Uint160, string, *wallet.Token, string) error) error {
	var accounts []*wallet.Account

	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, _, err := readWallet(ctx)
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
	var token *wallet.Token

	if name != "" {
		// Token was explicitly specified, let's try finding it, search in the wallet first.
		token, err = getMatchingToken(ctx, wall, name, standard)
		if err != nil {
			var h util.Uint160

			// Well-known hardcoded names/symbols.
			if standard == manifest.NEP17StandardName && (name == nativenames.Neo || name == "NEO") {
				h = neo.Hash
			} else if standard == manifest.NEP17StandardName && (name == nativenames.Gas || name == "GAS") {
				h = gas.Hash
			} else {
				// The last resort, maybe it's a direct hash or address.
				h, _ = flags.ParseAddress(name)
			}
			// If the hash is not found then it's some kind of named token, there is
			// no way for us to find it, but it's not an error, maybe we'll find it
			// in balances.
			if !h.Equals(util.Uint160{}) {
				// But if we have an exact hash, it must be correct.
				token, err = getTokenWithStandard(c, h, standard)
				if err != nil {
					return cli.NewExitError(fmt.Errorf("%q is not a valid %s token: %w", name, standard, err), 1)
				}
			}
		}
	}
	tokenID := ctx.String("id")
	if standard == manifest.NEP11StandardName {
		if len(tokenID) > 0 {
			_, err = hex.DecodeString(tokenID)
			if err != nil {
				return cli.NewExitError(fmt.Errorf("invalid token ID: %w", err), 1)
			}
		}
	}
	for k, acc := range accounts {
		if k != 0 {
			fmt.Fprintln(ctx.App.Writer)
		}
		fmt.Fprintf(ctx.App.Writer, "Account %s\n", acc.Address)

		err = accHandler(ctx, c, acc.ScriptHash(), name, token, tokenID)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	return nil
}

func decimalAmount(amount string, decimals int) string {
	if decimals != 0 {
		b, ok := new(big.Int).SetString(amount, 10)
		if ok {
			amount = fixedn.ToString(b, decimals)
		}
	}
	return amount
}

func printAssetBalance(ctx *cli.Context, balance result.NEP17Balance) {
	fmt.Fprintf(ctx.App.Writer, "%s: %s (%s)\n", balance.Symbol, balance.Name, balance.Asset.StringLE())
	fmt.Fprintf(ctx.App.Writer, "\tAmount : %s\n", decimalAmount(balance.Amount, balance.Decimals))
	fmt.Fprintf(ctx.App.Writer, "\tUpdated: %d\n", balance.LastUpdated)
}

func getMatchingToken(ctx *cli.Context, w *wallet.Wallet, name string, standard string) (*wallet.Token, error) {
	return getMatchingTokenAux(ctx, func(i int) *wallet.Token {
		return w.Extra.Tokens[i]
	}, len(w.Extra.Tokens), name, standard)
}

func tokenFromNEP17Balance(bal *result.NEP17Balance) *wallet.Token {
	return wallet.NewToken(bal.Asset, bal.Name, bal.Symbol, int64(bal.Decimals), manifest.NEP17StandardName)
}

func tokenFromNEP11Balance(bal *result.NEP11AssetBalance) *wallet.Token {
	return wallet.NewToken(bal.Asset, bal.Name, bal.Symbol, int64(bal.Decimals), manifest.NEP11StandardName)
}

func getMatchingTokenRPC(ctx *cli.Context, c *rpcclient.Client, addr util.Uint160, name string, standard string) (*wallet.Token, error) {
	switch standard {
	case manifest.NEP17StandardName:
		bs, err := c.GetNEP17Balances(addr)
		if err != nil {
			return nil, err
		}
		get := func(i int) *wallet.Token {
			return tokenFromNEP17Balance(&bs.Balances[i])
		}
		return getMatchingTokenAux(ctx, get, len(bs.Balances), name, standard)
	case manifest.NEP11StandardName:
		bs, err := c.GetNEP11Balances(addr)
		if err != nil {
			return nil, err
		}
		get := func(i int) *wallet.Token {
			return tokenFromNEP11Balance(&bs.Balances[i])
		}
		return getMatchingTokenAux(ctx, get, len(bs.Balances), name, standard)
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
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, _, err := openWallet(ctx, true)
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

	tok, err := getTokenWithStandard(c, tokenHash, standard)
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

func getTokenWithStandard(c *rpcclient.Client, hash util.Uint160, std string) (*wallet.Token, error) {
	token, err := neptoken.Info(c, hash)
	if err != nil {
		return nil, err
	}
	if token.Standard != std {
		return nil, fmt.Errorf("%s is not a %s token", hash.StringLE(), std)
	}
	return token, err
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
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, _, err := readWallet(ctx)
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
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, _, err := openWallet(ctx, true)
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
	wall, pass, err := readWallet(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	fromFlag := ctx.Generic("from").(*flags.Address)
	from, err := getDefaultAddress(fromFlag, wall)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	acc, err := options.GetUnlockedAccount(wall, from, pass)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	if ctx.NArg() == 0 {
		return cli.NewExitError("empty recipients list", 1)
	}
	var (
		recipients      []transferTarget
		cosignersSepPos = ctx.NArg() // `--` position.
	)
	for i := 0; i < ctx.NArg(); i++ {
		arg := ctx.Args().Get(i)
		if arg == cmdargs.CosignersSeparator {
			cosignersSepPos = i
			break
		}
	}
	cosigners, extErr := cmdargs.GetSignersFromContext(ctx, cosignersSepPos+1)
	if extErr != nil {
		return extErr
	}
	signersAccounts, err := cmdargs.GetSignersAccounts(acc, wall, cosigners, transaction.CalledByEntry)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid signers: %w", err), 1)
	}
	c, act, exitErr := options.GetRPCWithActor(gctx, ctx, signersAccounts)
	if exitErr != nil {
		return exitErr
	}

	cache := make(map[string]*wallet.Token)
	for i := 0; i < cosignersSepPos; i++ {
		arg := ctx.Args().Get(i)
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
		recipients = append(recipients, transferTarget{
			Token:   token.Hash,
			Address: addr,
			Amount:  amount.Int64(),
			Data:    nil,
		})
	}

	tx, err := makeMultiTransferNEP17(act, recipients)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't make transaction: %w", err), 1)
	}
	return txctx.SignAndSend(ctx, act, acc, tx)
}

func transferNEP17(ctx *cli.Context) error {
	return transferNEP(ctx, manifest.NEP17StandardName)
}

func transferNEP(ctx *cli.Context, standard string) error {
	var tx *transaction.Transaction

	wall, pass, err := readWallet(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	fromFlag := ctx.Generic("from").(*flags.Address)
	from, err := getDefaultAddress(fromFlag, wall)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	acc, err := options.GetUnlockedAccount(wall, from, pass)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	cosignersOffset, data, extErr := cmdargs.GetDataFromContext(ctx)
	if extErr != nil {
		return extErr
	}
	cosigners, extErr := cmdargs.GetSignersFromContext(ctx, cosignersOffset)
	if extErr != nil {
		return extErr
	}
	signersAccounts, err := cmdargs.GetSignersAccounts(acc, wall, cosigners, transaction.CalledByEntry)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid signers: %w", err), 1)
	}

	c, act, exitErr := options.GetRPCWithActor(gctx, ctx, signersAccounts)
	if exitErr != nil {
		return exitErr
	}

	toFlag := ctx.Generic("to").(*flags.Address)
	if !toFlag.IsSet {
		return cli.NewExitError(errors.New("missing receiver address (--to)"), 1)
	}
	to := toFlag.Uint160()
	token, err := getMatchingToken(ctx, wall, ctx.String("token"), standard)
	if err != nil {
		token, err = getMatchingTokenRPC(ctx, c, from, ctx.String("token"), standard)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't fetch matching token from RPC-node: %w", err), 1)
		}
	}

	amountArg := ctx.String("amount")
	amount, err := fixedn.FromString(amountArg, int(token.Decimals))
	// It's OK for NEP-11 transfer to not have amount set.
	if err != nil && (standard == manifest.NEP17StandardName || amountArg != "") {
		return cli.NewExitError(fmt.Errorf("invalid amount: %w", err), 1)
	}
	switch standard {
	case manifest.NEP17StandardName:
		n17 := nep17.New(act, token.Hash)
		tx, err = n17.TransferUnsigned(act.Sender(), to, amount, data)
	case manifest.NEP11StandardName:
		tokenID := ctx.String("id")
		if tokenID == "" {
			return cli.NewExitError(errors.New("token ID should be specified"), 1)
		}
		tokenIDBytes, terr := hex.DecodeString(tokenID)
		if terr != nil {
			return cli.NewExitError(fmt.Errorf("invalid token ID: %w", terr), 1)
		}
		if amountArg == "" {
			n11 := nep11.NewNonDivisible(act, token.Hash)
			tx, err = n11.TransferUnsigned(to, tokenIDBytes, data)
		} else {
			n11 := nep11.NewDivisible(act, token.Hash)
			tx, err = n11.TransferDUnsigned(act.Sender(), to, amount, tokenIDBytes, data)
		}
	default:
		return cli.NewExitError(fmt.Errorf("unsupported token standard %s", standard), 1)
	}
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't make transaction: %w", err), 1)
	}

	return txctx.SignAndSend(ctx, act, acc, tx)
}

func makeMultiTransferNEP17(act *actor.Actor, recipients []transferTarget) (*transaction.Transaction, error) {
	scr := smartcontract.NewBuilder()
	for i := range recipients {
		scr.InvokeWithAssert(recipients[i].Token, "transfer", act.Sender(),
			recipients[i].Address, recipients[i].Amount, recipients[i].Data)
	}
	script, err := scr.Script()
	if err != nil {
		return nil, err
	}
	return act.MakeUnsignedRun(script, nil)
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
