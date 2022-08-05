package smartcontract

import (
	"fmt"
	"os"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

var generateWrapperCmd = cli.Command{
	Name:        "generate-wrapper",
	Usage:       "generate wrapper to use in other contracts",
	UsageText:   "neo-go contract generate-wrapper --manifest <file.json> --out <file.go> --hash <hash>",
	Description: ``,
	Action:      contractGenerateWrapper,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Usage: "Configuration file to use",
		},
		cli.StringFlag{
			Name:  "manifest, m",
			Usage: "Read contract manifest (*.manifest.json) file",
		},
		cli.StringFlag{
			Name:  "out, o",
			Usage: "Output of the compiled contract",
		},
		cli.StringFlag{
			Name:  "hash",
			Usage: "Smart-contract hash",
		},
	},
}

// contractGenerateWrapper deploys contract.
func contractGenerateWrapper(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	m, _, err := readManifest(ctx.String("manifest"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't read contract manifest: %w", err), 1)
	}

	cfg := binding.NewConfig()
	if cfgPath := ctx.String("config"); cfgPath != "" {
		bs, err := os.ReadFile(cfgPath)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't read config file: %w", err), 1)
		}
		err = yaml.Unmarshal(bs, &cfg)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't parse config file: %w", err), 1)
		}
	}

	cfg.Manifest = m

	h, err := util.Uint160DecodeStringLE(strings.TrimPrefix(ctx.String("hash"), "0x"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid contract hash: %w", err), 1)
	}
	cfg.Hash = h

	f, err := os.Create(ctx.String("out"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't create output file: %w", err), 1)
	}
	defer f.Close()

	cfg.Output = f

	err = binding.Generate(cfg)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("error during generation: %w", err), 1)
	}
	return nil
}
