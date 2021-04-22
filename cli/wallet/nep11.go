package wallet

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/urfave/cli"
)

func newNEP11Commands() []cli.Command {
	return []cli.Command{
		{
			Name:      "import",
			Usage:     "import NEP11 token to a wallet",
			UsageText: "import --wallet <path> --rpc-endpoint <node> --timeout <time> --token <hash>",
			Action:    importNEP11Token,
			Flags:     importFlags,
		},
	}
}

func importNEP11Token(ctx *cli.Context) error {
	return importNEPToken(ctx, manifest.NEP11StandardName)
}
