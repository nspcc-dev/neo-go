package smartcontract

import (
	"fmt"
	"os"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/rpcbinding"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

var generatorFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "config, c",
		Usage: "Configuration file to use",
	},
	cli.StringFlag{
		Name:     "manifest, m",
		Required: true,
		Usage:    "Read contract manifest (*.manifest.json) file",
	},
	cli.StringFlag{
		Name:     "out, o",
		Required: true,
		Usage:    "Output of the compiled wrapper",
	},
	cli.StringFlag{
		Name:  "hash",
		Usage: "Smart-contract hash",
	},
}

var generateWrapperCmd = cli.Command{
	Name:        "generate-wrapper",
	Usage:       "generate wrapper to use in other contracts",
	UsageText:   "neo-go contract generate-wrapper --manifest <file.json> --out <file.go> --hash <hash> [--config <config>]",
	Description: ``,
	Action:      contractGenerateWrapper,
	Flags:       generatorFlags,
}

var generateRPCWrapperCmd = cli.Command{
	Name:      "generate-rpcwrapper",
	Usage:     "generate RPC wrapper to use for data reads",
	UsageText: "neo-go contract generate-rpcwrapper --manifest <file.json> --out <file.go> [--hash <hash>] [--config <config>]",
	Action:    contractGenerateRPCWrapper,
	Flags:     generatorFlags,
}

func contractGenerateWrapper(ctx *cli.Context) error {
	return contractGenerateSomething(ctx, binding.Generate, false)
}

func contractGenerateRPCWrapper(ctx *cli.Context) error {
	return contractGenerateSomething(ctx, rpcbinding.Generate, true)
}

// contractGenerateSomething reads generator parameters and calls the given callback.
func contractGenerateSomething(ctx *cli.Context, cb func(binding.Config) error, allowEmptyHash bool) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	var (
		h   util.Uint160
		err error
	)
	if hStr := ctx.String("hash"); len(hStr) != 0 {
		h, err = util.Uint160DecodeStringLE(strings.TrimPrefix(hStr, "0x"))
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid contract hash: %w", err), 1)
		}
	} else if !allowEmptyHash {
		return cli.NewExitError("contract hash must be provided via --hash flag", 1)
	}
	m, _, err := readManifest(ctx.String("manifest"), h)
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
	cfg.Hash = h

	f, err := os.Create(ctx.String("out"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't create output file: %w", err), 1)
	}
	defer f.Close()

	cfg.Output = f

	err = cb(cfg)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("error during generation: %w", err), 1)
	}
	return nil
}
