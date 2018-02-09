package main

import (
	"strings"

	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/urfave/cli"
)

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
