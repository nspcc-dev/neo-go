package wallet

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

func newNEP5Commands() []cli.Command {
	return []cli.Command{
		{
			Name:      "import",
			Usage:     "import NEP5 token to a wallet",
			UsageText: "import --path <path> --rpc <node> --token <hash>",
			Action:    importNEP5Token,
			Flags: []cli.Flag{
				walletPathFlag,
				rpcFlag,
				cli.StringFlag{
					Name:  "token",
					Usage: "Token contract hash in LE",
				},
			},
		},
	}
}

func importNEP5Token(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	tokenHash, err := util.Uint160DecodeStringLE(ctx.String("token"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid token contract hash: %v", err), 1)
	}

	for _, t := range wall.Extra.Tokens {
		if t.Hash.Equals(tokenHash) {
			printTokenInfo(t)
			return cli.NewExitError("token already exists", 1)
		}
	}

	gctx, cancel := getGoContext(ctx)
	defer cancel()

	c, err := client.New(gctx, ctx.String("rpc"), client.Options{})
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tok, err := c.NEP5TokenInfo(tokenHash)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't receive token info: %v", err), 1)
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
	fmt.Printf("Address: %s\n", tok.Address)
}
