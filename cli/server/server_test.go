package server

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/network/metrics"
	"github.com/nspcc-dev/neo-go/pkg/rpc"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

func TestGetConfigFromContext(t *testing.T) {
	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	set.String("config-path", "../../config", "")
	set.Bool("testnet", true, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	cfg, err := getConfigFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, netmode.TestNet, cfg.ProtocolConfiguration.Magic)
}

func TestHandleLoggingParams(t *testing.T) {
	testLog, err := ioutil.TempFile("./", "*.log")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Remove(testLog.Name()))
	})

	t.Run("default", func(t *testing.T) {
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		cfg := config.ApplicationConfiguration{
			LogPath: testLog.Name(),
		}
		logger, err := handleLoggingParams(ctx, cfg)
		require.NoError(t, err)
		require.True(t, logger.Core().Enabled(zap.InfoLevel))
		require.False(t, logger.Core().Enabled(zap.DebugLevel))
	})

	t.Run("debug", func(t *testing.T) {
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.Bool("debug", true, "")
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		cfg := config.ApplicationConfiguration{
			LogPath: testLog.Name(),
		}
		logger, err := handleLoggingParams(ctx, cfg)
		require.NoError(t, err)
		require.True(t, logger.Core().Enabled(zap.InfoLevel))
		require.True(t, logger.Core().Enabled(zap.DebugLevel))
	})
}

func TestInitBCWithMetrics(t *testing.T) {
	d, err := ioutil.TempDir("./", "")
	require.NoError(t, err)
	err = os.Chdir(d)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.Chdir("..")
		require.NoError(t, err)
		os.RemoveAll(d)
	})

	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	set.String("config-path", "../../../config", "")
	set.Bool("testnet", true, "")
	set.Bool("debug", true, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	cfg, err := getConfigFromContext(ctx)
	require.NoError(t, err)
	logger, err := handleLoggingParams(ctx, cfg.ApplicationConfiguration)
	require.NoError(t, err)
	chain, prometheus, pprof, err := initBCWithMetrics(cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		chain.Close()
		prometheus.ShutDown()
		pprof.ShutDown()
	})
	require.Equal(t, netmode.TestNet, chain.GetConfig().Magic)
}

func TestDumpDB(t *testing.T) {
	t.Run("too low chain", func(t *testing.T) {
		d, err := ioutil.TempDir("./", "")
		require.NoError(t, err)
		err = os.Chdir(d)
		require.NoError(t, err)
		t.Cleanup(func() {
			err = os.Chdir("..")
			require.NoError(t, err)
			os.RemoveAll(d)
		})
		testDump := "file.acc"
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.String("config-path", "../../../config", "")
		set.Bool("privnet", true, "")
		set.Bool("debug", true, "")
		set.Int("start", 0, "")
		set.Int("count", 5, "")
		set.String("out", testDump, "")
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		err = dumpDB(ctx)
		require.Error(t, err)
	})

	t.Run("positive", func(t *testing.T) {
		d, err := ioutil.TempDir("./", "")
		require.NoError(t, err)
		err = os.Chdir(d)
		require.NoError(t, err)
		t.Cleanup(func() {
			err = os.Chdir("..")
			require.NoError(t, err)
			os.RemoveAll(d)
		})
		testDump := "file.acc"
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.String("config-path", "../../../config", "")
		set.Bool("privnet", true, "")
		set.Bool("debug", true, "")
		set.Int("start", 0, "")
		set.Int("count", 1, "")
		set.String("out", testDump, "")
		ctx := cli.NewContext(cli.NewApp(), set, nil)
		err = dumpDB(ctx)
		require.NoError(t, err)
	})
}

func TestRestoreDB(t *testing.T) {
	d, err := ioutil.TempDir("./", "")
	require.NoError(t, err)
	testDump := "file1.acc"
	saveDump := "file2.acc"
	err = os.Chdir(d)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.Chdir("..")
		require.NoError(t, err)
		os.RemoveAll(d)
	})

	//dump first
	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	set.String("config-path", "../../../config", "")
	set.Bool("privnet", true, "")
	set.Bool("debug", true, "")
	set.Int("start", 0, "")
	set.Int("count", 1, "")
	set.String("out", testDump, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	err = dumpDB(ctx)
	require.NoError(t, err)

	// and then restore
	set.String("in", testDump, "")
	set.String("dump", saveDump, "")
	require.NoError(t, restoreDB(ctx))
}

func TestConfigureAddresses(t *testing.T) {
	defaultAddress := "http://127.0.0.1:10333"
	customAddress := "http://127.0.0.1:10334"

	t.Run("default addresses", func(t *testing.T) {
		cfg := &config.ApplicationConfiguration{
			Address: defaultAddress,
		}
		configureAddresses(cfg)
		require.Equal(t, defaultAddress, cfg.RPC.Address)
		require.Equal(t, defaultAddress, cfg.Prometheus.Address)
		require.Equal(t, defaultAddress, cfg.Pprof.Address)
	})

	t.Run("custom RPC address", func(t *testing.T) {
		cfg := &config.ApplicationConfiguration{
			Address: defaultAddress,
			RPC: rpc.Config{
				Address: customAddress,
			},
		}
		configureAddresses(cfg)
		require.Equal(t, cfg.RPC.Address, customAddress)
		require.Equal(t, cfg.Prometheus.Address, defaultAddress)
		require.Equal(t, cfg.Pprof.Address, defaultAddress)
	})

	t.Run("custom Pprof address", func(t *testing.T) {
		cfg := &config.ApplicationConfiguration{
			Address: defaultAddress,
			Pprof: metrics.Config{
				Address: customAddress,
			},
		}
		configureAddresses(cfg)
		require.Equal(t, cfg.RPC.Address, defaultAddress)
		require.Equal(t, cfg.Prometheus.Address, defaultAddress)
		require.Equal(t, cfg.Pprof.Address, customAddress)
	})

	t.Run("custom Prometheus address", func(t *testing.T) {
		cfg := &config.ApplicationConfiguration{
			Address: defaultAddress,
			Prometheus: metrics.Config{
				Address: customAddress,
			},
		}
		configureAddresses(cfg)
		require.Equal(t, cfg.RPC.Address, defaultAddress)
		require.Equal(t, cfg.Prometheus.Address, customAddress)
		require.Equal(t, cfg.Pprof.Address, defaultAddress)
	})
}

func TestInitBlockChain(t *testing.T) {
	t.Run("bad storage", func(t *testing.T) {
		_, err := initBlockChain(config.Config{}, nil)
		require.Error(t, err)
	})

	t.Run("empty logger", func(t *testing.T) {
		_, err := initBlockChain(config.Config{
			ApplicationConfiguration: config.ApplicationConfiguration{
				DBConfiguration: storage.DBConfiguration{
					Type: "inmemory",
				},
			},
		}, nil)
		require.Error(t, err)
	})
}
