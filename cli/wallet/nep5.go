package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/context"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

// validUntilBlockIncrement is the number of extra blocks to add to an exported transaction
const validUntilBlockIncrement = 50

var (
	neoToken = wallet.NewToken(client.NeoContractHash, "NEO", "neo", 0)
	gasToken = wallet.NewToken(client.GasContractHash, "GAS", "gas", 8)
)

var (
	tokenFlag = cli.StringFlag{
		Name:  "token",
		Usage: "Token to use",
	}
	gasFlag = flags.Fixed8Flag{
		Name:  "gas",
		Usage: "Amount of GAS to attach to a tx",
	}
)

func newNEP5Commands() []cli.Command {
	balanceFlags := []cli.Flag{
		walletPathFlag,
		tokenFlag,
		cli.StringFlag{
			Name:  "addr",
			Usage: "Address to use",
		},
	}
	balanceFlags = append(balanceFlags, options.RPC...)
	importFlags := []cli.Flag{
		walletPathFlag,
		cli.StringFlag{
			Name:  "token",
			Usage: "Token contract hash in LE",
		},
	}
	importFlags = append(importFlags, options.RPC...)
	transferFlags := []cli.Flag{
		walletPathFlag,
		outFlag,
		fromAddrFlag,
		toAddrFlag,
		tokenFlag,
		gasFlag,
		cli.StringFlag{
			Name:  "amount",
			Usage: "Amount of asset to send",
		},
	}
	transferFlags = append(transferFlags, options.RPC...)
	multiTransferFlags := []cli.Flag{
		walletPathFlag,
		outFlag,
		fromAddrFlag,
		gasFlag,
	}
	multiTransferFlags = append(multiTransferFlags, options.RPC...)
	return []cli.Command{
		{
			Name:      "balance",
			Usage:     "get address balance",
			UsageText: "balance --wallet <path> --rpc-endpoint <node> --timeout <time> --addr <addr> [--token <hash-or-name>]",
			Action:    getNEP5Balance,
			Flags:     balanceFlags,
		},
		{
			Name:      "import",
			Usage:     "import NEP5 token to a wallet",
			UsageText: "import --wallet <path> --rpc-endpoint <node> --timeout <time> --token <hash>",
			Action:    importNEP5Token,
			Flags:     importFlags,
		},
		{
			Name:      "info",
			Usage:     "print imported NEP5 token info",
			UsageText: "print --wallet <path> [--token <hash-or-name>]",
			Action:    printNEP5Info,
			Flags: []cli.Flag{
				walletPathFlag,
				cli.StringFlag{
					Name:  "token",
					Usage: "Token name or hash",
				},
			},
		},
		{
			Name:      "remove",
			Usage:     "remove NEP5 token from the wallet",
			UsageText: "remove --wallet <path> <hash-or-name>",
			Action:    removeNEP5Token,
			Flags: []cli.Flag{
				walletPathFlag,
				cli.StringFlag{
					Name:  "token",
					Usage: "Token name or hash",
				},
				forceFlag,
			},
		},
		{
			Name:      "transfer",
			Usage:     "transfer NEP5 tokens",
			UsageText: "transfer --wallet <path> --rpc-endpoint <node> --timeout <time> --from <addr> --to <addr> --token <hash> --amount string",
			Action:    transferNEP5,
			Flags:     transferFlags,
		},
		{
			Name:  "multitransfer",
			Usage: "transfer NEP5 tokens to multiple recepients",
			UsageText: `multitransfer --wallet <path> --rpc-endpoint <node> --timeout <time> --from <addr>` +
				` <token1>:<addr1>:<amount1> [<token2>:<addr2>:<amount2> [...]]`,
			Action: multiTransferNEP5,
			Flags:  multiTransferFlags,
		},
	}
}

func getNEP5Balance(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addr := ctx.String("addr")
	addrHash, err := address.StringToUint160(addr)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid address: %w", err), 1)
	}
	acc := wall.GetAccount(addrHash)
	if acc == nil {
		return cli.NewExitError(fmt.Errorf("can't find account for the address: %s", addr), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	var token *wallet.Token
	name := ctx.String("token")
	if name != "" {
		token, err = getMatchingToken(wall, name)
		if err != nil {
			token, err = getMatchingTokenRPC(c, addrHash, name)
			if err != nil {
				return cli.NewExitError(err, 1)
			}
		}
	}

	balances, err := c.GetNEP5Balances(addrHash)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	for i := range balances.Balances {
		asset := balances.Balances[i].Asset
		if name != "" && !token.Hash.Equals(asset) {
			continue
		}
		fmt.Printf("TokenHash: %s\n", asset.StringLE())
		fmt.Printf("\tAmount : %s\n", balances.Balances[i].Amount)
		fmt.Printf("\tUpdated: %d\n", balances.Balances[i].LastUpdated)
	}
	return nil
}

func getMatchingToken(w *wallet.Wallet, name string) (*wallet.Token, error) {
	switch strings.ToLower(name) {
	case "neo":
		return neoToken, nil
	case "gas":
		return gasToken, nil
	}
	return getMatchingTokenAux(func(i int) *wallet.Token {
		return w.Extra.Tokens[i]
	}, len(w.Extra.Tokens), name)
}

func getMatchingTokenRPC(c *client.Client, addr util.Uint160, name string) (*wallet.Token, error) {
	bs, err := c.GetNEP5Balances(addr)
	if err != nil {
		return nil, err
	}
	get := func(i int) *wallet.Token {
		t, _ := c.NEP5TokenInfo(bs.Balances[i].Asset)
		return t
	}
	return getMatchingTokenAux(get, len(bs.Balances), name)
}

func getMatchingTokenAux(get func(i int) *wallet.Token, n int, name string) (*wallet.Token, error) {
	var token *wallet.Token
	var count int
	for i := 0; i < n; i++ {
		t := get(i)
		if t != nil && (t.Name == name || t.Symbol == name || t.Address() == name || t.Hash.StringLE() == name) {
			if count == 1 {
				printTokenInfo(token)
				printTokenInfo(t)
				return nil, errors.New("multiple matching tokens found")
			}
			count++
			token = t
		}
	}
	if count == 0 {
		return nil, errors.New("token was not found")
	}
	return token, nil
}

func importNEP5Token(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	tokenHash, err := util.Uint160DecodeStringLE(ctx.String("token"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid token contract hash: %w", err), 1)
	}

	for _, t := range wall.Extra.Tokens {
		if t.Hash.Equals(tokenHash) {
			printTokenInfo(t)
			return cli.NewExitError("token already exists", 1)
		}
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	tok, err := c.NEP5TokenInfo(tokenHash)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't receive token info: %w", err), 1)
	}

	wall.AddToken(tok)
	if err := wall.Save(); err != nil {
		return cli.NewExitError(err, 1)
	}
	printTokenInfo(tok)
	return nil
}

func printTokenInfo(tok *wallet.Token) {
	fmt.Printf("Name:\t%s\n", tok.Name)
	fmt.Printf("Symbol:\t%s\n", tok.Symbol)
	fmt.Printf("Hash:\t%s\n", tok.Hash.StringLE())
	fmt.Printf("Decimals: %d\n", tok.Decimals)
	fmt.Printf("Address: %s\n", tok.Address())
}

func printNEP5Info(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	if name := ctx.String("token"); name != "" {
		token, err := getMatchingToken(wall, name)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		printTokenInfo(token)
		return nil
	}

	for i, t := range wall.Extra.Tokens {
		if i > 0 {
			fmt.Println()
		}
		printTokenInfo(t)
	}
	return nil
}

func removeNEP5Token(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	name := ctx.Args().First()
	if name == "" {
		return cli.NewExitError("token must be specified", 1)
	}
	token, err := getMatchingToken(wall, name)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if !ctx.Bool("force") {
		if ok := askForConsent(); !ok {
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

func multiTransferNEP5(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	fromFlag := ctx.Generic("from").(*flags.Address)
	from := fromFlag.Uint160()
	acc, err := getDecryptedAccount(wall, from)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	if ctx.NArg() == 0 {
		return cli.NewExitError("empty recepients list", 1)
	}
	var recepients []client.TransferTarget
	cache := make(map[string]*wallet.Token)
	for i := 0; i < ctx.NArg(); i++ {
		arg := ctx.Args().Get(i)
		ss := strings.SplitN(arg, ":", 3)
		if len(ss) != 3 {
			return cli.NewExitError("send format must be '<token>:<addr>:<amount>", 1)
		}
		token, ok := cache[ss[0]]
		if !ok {
			token, err = getMatchingToken(wall, ss[0])
			if err != nil {
				fmt.Println("Can't find matching token in the wallet. Querying RPC-node for balances.")
				token, err = getMatchingTokenRPC(c, from, ctx.String("token"))
				if err != nil {
					return cli.NewExitError(err, 1)
				}
			}
		}
		cache[ss[0]] = token
		addr, err := address.StringToUint160(ss[1])
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid address: '%s'", ss[1]), 1)
		}
		amount, err := util.FixedNFromString(ss[2], int(token.Decimals))
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid amount: %w", err), 1)
		}
		recepients = append(recepients, client.TransferTarget{
			Token:   token.Hash,
			Address: addr,
			Amount:  amount,
		})
	}

	return signAndSendTransfer(ctx, c, acc, recepients)
}

func transferNEP5(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	fromFlag := ctx.Generic("from").(*flags.Address)
	from := fromFlag.Uint160()
	acc, err := getDecryptedAccount(wall, from)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	toFlag := ctx.Generic("to").(*flags.Address)
	to := toFlag.Uint160()
	token, err := getMatchingToken(wall, ctx.String("token"))
	if err != nil {
		fmt.Println("Can't find matching token in the wallet. Querying RPC-node for balances.")
		token, err = getMatchingTokenRPC(c, from, ctx.String("token"))
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	amount, err := util.FixedNFromString(ctx.String("amount"), int(token.Decimals))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid amount: %w", err), 1)
	}

	return signAndSendTransfer(ctx, c, acc, []client.TransferTarget{{
		Token:   token.Hash,
		Address: to,
		Amount:  amount,
	}})
}

func signAndSendTransfer(ctx *cli.Context, c *client.Client, acc *wallet.Account, recepients []client.TransferTarget) error {
	gas := flags.Fixed8FromContext(ctx, "gas")

	tx, err := c.CreateNEP5MultiTransferTx(acc, int64(gas), recepients...)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if outFile := ctx.String("out"); outFile != "" {
		// avoid fast transaction expiration
		tx.ValidUntilBlock += validUntilBlockIncrement
		priv := acc.PrivateKey()
		pub := priv.PublicKey()
		sign := priv.Sign(tx.GetSignedPart())
		scCtx := context.NewParameterContext("Neo.Core.ContractTransaction", tx)
		if err := scCtx.AddSignature(acc.Contract, pub, sign); err != nil {
			return cli.NewExitError(fmt.Errorf("can't add signature: %w", err), 1)
		} else if data, err := json.Marshal(scCtx); err != nil {
			return cli.NewExitError(fmt.Errorf("can't marshal tx to JSON: %w", err), 1)
		} else if err := ioutil.WriteFile(outFile, data, 0644); err != nil {
			return cli.NewExitError(fmt.Errorf("can't write tx to file: %w", err), 1)
		}
	} else {
		_ = acc.SignTx(tx)
		res, err := c.SendRawTransaction(tx)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		fmt.Println(res.StringLE())
		return nil
	}

	fmt.Println(tx.Hash().StringLE())
	return nil
}
