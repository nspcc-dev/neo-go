package server

import (
	"strings"

	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/urfave/cli"
)

// NewCommand creates a new Node command.
func NewCommand() cli.Command {
	return cli.Command{
		Name:   "node",
		Usage:  "start a NEO node",
		Action: startServer,
		Flags: []cli.Flag{
			cli.IntFlag{Name: "tcp"},
			cli.IntFlag{Name: "rpc"},
			cli.StringFlag{Name: "seed"},
			cli.BoolFlag{Name: "privnet, p"},
			cli.BoolFlag{Name: "mainnet, m"},
			cli.BoolFlag{Name: "testnet, t"},
		},
	}
}

func startServer(ctx *cli.Context) error {
	opts := network.StartOpts{
		Seeds: parseSeeds(ctx.String("seed")),
		TCP:   ctx.Int("tcp"),
		RPC:   ctx.Int("rpc"),
	}

	net := network.ModePrivNet
	if ctx.Bool("testnet") {
		net = network.ModeTestNet
	}
	if ctx.Bool("mainnet") {
		net = network.ModeMainNet
	}

	s := network.NewServer(net)
	s.Start(opts)
	return nil
}

func parseSeeds(s string) []string {
	if len(s) == 0 {
		return nil
	}
	seeds := strings.Split(s, ",")
	if len(seeds) == 0 {
		return nil
	}
	return seeds
}
