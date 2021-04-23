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
