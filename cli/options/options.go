/*
Package options contains a set of common CLI options and helper functions to use them.
*/
package options

import (
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/urfave/cli"
)

// Network is a set of flags for choosing the network to operate on
// (privnet/mainnet/testnet).
var Network = []cli.Flag{
	cli.BoolFlag{Name: "privnet, p"},
	cli.BoolFlag{Name: "mainnet, m"},
	cli.BoolFlag{Name: "testnet, t"},
}

// GetNetwork examines Context's flags and returns the appropriate network. It
// defaults to PrivNet if no flags are given.
func GetNetwork(ctx *cli.Context) netmode.Magic {
	var net = netmode.PrivNet
	if ctx.Bool("testnet") {
		net = netmode.TestNet
	}
	if ctx.Bool("mainnet") {
		net = netmode.MainNet
	}
	return net
}
