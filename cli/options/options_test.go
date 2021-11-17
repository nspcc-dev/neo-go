package options

import (
	"flag"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestGetNetwork(t *testing.T) {
	t.Run("privnet", func(t *testing.T) {
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		require.Equal(t, netmode.PrivNet, GetNetwork(ctx))
	})

	t.Run("testnet", func(t *testing.T) {
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.Bool("testnet", true, "")
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		require.Equal(t, netmode.TestNet, GetNetwork(ctx))
	})

	t.Run("mainnet", func(t *testing.T) {
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.Bool("mainnet", true, "")
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		require.Equal(t, netmode.MainNet, GetNetwork(ctx))
	})
}

func TestGetTimeoutContext(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		start := time.Now()
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		actualCtx, _ := GetTimeoutContext(ctx)
		end := time.Now().Add(DefaultTimeout)
		dl, _ := actualCtx.Deadline()
		require.True(t, start.Before(dl) && (dl.Before(end) || dl.Equal(end)))
	})

	t.Run("set", func(t *testing.T) {
		start := time.Now()
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.Duration("timeout", time.Duration(20), "")
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		actualCtx, _ := GetTimeoutContext(ctx)
		end := time.Now().Add(time.Nanosecond * 20)
		dl, _ := actualCtx.Deadline()
		require.True(t, start.Before(dl) && (dl.Before(end) || dl.Equal(end)))
	})
}
