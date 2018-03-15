package server

import (
	"fmt"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/util"
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
			cli.BoolFlag{Name: "relay, r"},
			cli.StringFlag{Name: "seed"},
			cli.StringFlag{Name: "dbfile"},
			cli.BoolFlag{Name: "privnet, p"},
			cli.BoolFlag{Name: "mainnet, m"},
			cli.BoolFlag{Name: "testnet, t"},
		},
	}
}

func startServer(ctx *cli.Context) error {
	net := network.ModePrivNet
	if ctx.Bool("testnet") {
		net = network.ModeTestNet
	}
	if ctx.Bool("mainnet") {
		net = network.ModeMainNet
	}

	cfg := network.Config{
		UserAgent: "/NEO-GO:0.26.0/",
		ListenTCP: uint16(ctx.Int("tcp")),
		Seeds:     parseSeeds(ctx.String("seed")),
		Net:       net,
		Relay:     ctx.Bool("relay"),
	}

	chain, err := newBlockchain(net, ctx.String("dbfile"))
	if err != nil {
		err = fmt.Errorf("could not initialize blockhain: %s", err)
		return cli.NewExitError(err, 1)
	}

	s := network.NewServer(cfg, chain)
	s.Start()
	return nil
}

func newBlockchain(net network.NetMode, path string) (*core.Blockchain, error) {
	var startHash util.Uint256
	if net == network.ModePrivNet {
		startHash = core.GenesisHashPrivNet()
	}
	if net == network.ModeTestNet {
		startHash = core.GenesisHashTestNet()
	}
	if net == network.ModeMainNet {
		startHash = core.GenesisHashMainNet()
	}

	// Hardcoded for now.
	store, err := core.NewLevelDBStore(path, nil)
	if err != nil {
		return nil, err
	}

	return core.NewBlockchain(store, startHash)
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
