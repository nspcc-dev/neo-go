/*
Package options contains a set of common CLI options and helper functions to use them.
*/
package options

import (
	"context"
	"errors"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/urfave/cli"
)

// DefaultTimeout is the default timeout used for RPC requests.
const DefaultTimeout = 10 * time.Second

// RPCEndpointFlag is a long flag name for an RPC endpoint. It can be used to
// check for flag presence in the context.
const RPCEndpointFlag = "rpc-endpoint"

// Network is a set of flags for choosing the network to operate on
// (privnet/mainnet/testnet).
var Network = []cli.Flag{
	cli.BoolFlag{Name: "privnet, p"},
	cli.BoolFlag{Name: "mainnet, m"},
	cli.BoolFlag{Name: "testnet, t"},
	cli.BoolFlag{Name: "unittest", Hidden: true},
}

// RPC is a set of flags used for RPC connections (endpoint and timeout).
var RPC = []cli.Flag{
	cli.StringFlag{
		Name:  RPCEndpointFlag + ", r",
		Usage: "RPC node address",
	},
	cli.DurationFlag{
		Name:  "timeout, s",
		Value: DefaultTimeout,
		Usage: "Timeout for the operation",
	},
}

var errNoEndpoint = errors.New("no RPC endpoint specified, use option '--" + RPCEndpointFlag + "' or '-r'")

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
	if ctx.Bool("unittest") {
		net = netmode.UnitTestNet
	}
	return net
}

// GetTimeoutContext returns a context.Context with the default or a user-set timeout.
func GetTimeoutContext(ctx *cli.Context) (context.Context, func()) {
	dur := ctx.Duration("timeout")
	if dur == 0 {
		dur = DefaultTimeout
	}
	return context.WithTimeout(context.Background(), dur)
}

// GetRPCClient returns an RPC client instance for the given Context.
func GetRPCClient(gctx context.Context, ctx *cli.Context) (*rpcclient.Client, cli.ExitCoder) {
	endpoint := ctx.String(RPCEndpointFlag)
	if len(endpoint) == 0 {
		return nil, cli.NewExitError(errNoEndpoint, 1)
	}
	c, err := rpcclient.New(gctx, endpoint, rpcclient.Options{})
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	err = c.Init()
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	return c, nil
}
