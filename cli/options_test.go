package main

import (
	"flag"
	"testing"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestGetRPCClient(t *testing.T) {
	e := newExecutor(t, true)

	t.Run("no endpoint", func(t *testing.T) {
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		gctx, _ := options.GetTimeoutContext(ctx)
		_, ec := options.GetRPCClient(gctx, ctx)
		require.Equal(t, 1, ec.ExitCode())
	})

	t.Run("success", func(t *testing.T) {
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.String(options.RPCEndpointFlag, "http://"+e.RPC.Addr, "")
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		gctx, _ := options.GetTimeoutContext(ctx)
		_, ec := options.GetRPCClient(gctx, ctx)
		require.Nil(t, ec)
	})
}
