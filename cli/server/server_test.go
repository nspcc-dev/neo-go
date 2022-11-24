package server

import (
	"encoding/binary"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zapcore"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

// serverTestWD is the default working directory for server tests.
var serverTestWD string

func init() {
	var err error
	serverTestWD, err = os.Getwd()
	if err != nil {
		panic("can't get current working directory")
	}
}

func TestGetConfigFromContext(t *testing.T) {
	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	set.String("config-path", "../../config", "")
	set.Bool("testnet", true, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	cfg, err := options.GetConfigFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, netmode.TestNet, cfg.ProtocolConfiguration.Magic)
}

func TestHandleLoggingParams(t *testing.T) {
	d := t.TempDir()
	testLog := filepath.Join(d, "file.log")

	t.Run("logdir is a file", func(t *testing.T) {
		logfile := filepath.Join(d, "logdir")
		require.NoError(t, os.WriteFile(logfile, []byte{1, 2, 3}, os.ModePerm))
		cfg := config.ApplicationConfiguration{
			LogPath: filepath.Join(logfile, "file.log"),
		}
		_, lvl, closer, err := options.HandleLoggingParams(false, cfg)
		require.Error(t, err)
		require.Nil(t, lvl)
		require.Nil(t, closer)
	})

	t.Run("broken level", func(t *testing.T) {
		cfg := config.ApplicationConfiguration{
			LogPath:  testLog,
			LogLevel: "qwerty",
		}
		_, lvl, closer, err := options.HandleLoggingParams(false, cfg)
		require.Error(t, err)
		require.Nil(t, lvl)
		require.Nil(t, closer)
	})

	t.Run("default", func(t *testing.T) {
		cfg := config.ApplicationConfiguration{
			LogPath: testLog,
		}
		logger, lvl, closer, err := options.HandleLoggingParams(false, cfg)
		require.NotNil(t, lvl)
		require.NoError(t, err)
		t.Cleanup(func() {
			if closer != nil {
				require.NoError(t, closer())
			}
		})
		require.Equal(t, zapcore.InfoLevel, lvl.Level())
		require.True(t, logger.Core().Enabled(zapcore.InfoLevel))
		require.False(t, logger.Core().Enabled(zapcore.DebugLevel))
	})

	t.Run("warn", func(t *testing.T) {
		cfg := config.ApplicationConfiguration{
			LogPath:  testLog,
			LogLevel: "warn",
		}
		logger, lvl, closer, err := options.HandleLoggingParams(false, cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			if closer != nil {
				require.NoError(t, closer())
			}
		})
		require.Equal(t, zapcore.WarnLevel, lvl.Level())
		require.True(t, logger.Core().Enabled(zapcore.WarnLevel))
		require.False(t, logger.Core().Enabled(zapcore.InfoLevel))
	})

	t.Run("debug", func(t *testing.T) {
		cfg := config.ApplicationConfiguration{
			LogPath: testLog,
		}
		logger, lvl, closer, err := options.HandleLoggingParams(true, cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			if closer != nil {
				require.NoError(t, closer())
			}
		})
		require.Equal(t, zapcore.DebugLevel, lvl.Level())
		require.True(t, logger.Core().Enabled(zapcore.InfoLevel))
		require.True(t, logger.Core().Enabled(zapcore.DebugLevel))
	})
}

func TestInitBCWithMetrics(t *testing.T) {
	d := t.TempDir()
	err := os.Chdir(d)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(serverTestWD)) })

	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	set.String("config-path", filepath.Join(serverTestWD, "..", "..", "config"), "")
	set.Bool("testnet", true, "")
	set.Bool("debug", true, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	cfg, err := options.GetConfigFromContext(ctx)
	require.NoError(t, err)
	logger, _, closer, err := options.HandleLoggingParams(true, cfg.ApplicationConfiguration)
	require.NoError(t, err)
	t.Cleanup(func() {
		if closer != nil {
			require.NoError(t, closer())
		}
	})

	t.Run("bad store", func(t *testing.T) {
		_, _, _, err = initBCWithMetrics(config.Config{}, logger)
		require.Error(t, err)
	})

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
	testDump := "file.acc"

	t.Run("too low chain", func(t *testing.T) {
		d := t.TempDir()
		err := os.Chdir(d)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, os.Chdir(serverTestWD)) })
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.String("config-path", filepath.Join(serverTestWD, "..", "..", "config"), "")
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
		d := t.TempDir()
		err := os.Chdir(d)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, os.Chdir(serverTestWD)) })
		set := flag.NewFlagSet("flagSet", flag.ExitOnError)
		set.String("config-path", filepath.Join(serverTestWD, "..", "..", "config"), "")
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
	d := t.TempDir()
	testDump := "file1.acc"
	saveDump := "file2.acc"
	err := os.Chdir(d)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(serverTestWD)) })

	// dump first
	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	goodCfg := filepath.Join(serverTestWD, "..", "..", "config")
	cfgPath := set.String("config-path", goodCfg, "")
	set.Bool("privnet", true, "")
	set.Bool("debug", true, "")
	set.Int("start", 0, "")
	set.Int("count", 1, "")
	set.String("out", testDump, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	err = dumpDB(ctx)
	require.NoError(t, err)

	// and then restore
	t.Run("invalid config", func(t *testing.T) {
		*cfgPath = filepath.Join(serverTestWD, "..", "..", "config_invalid")
		require.Error(t, restoreDB(ctx))
	})
	t.Run("invalid logger path", func(t *testing.T) {
		badCfgDir := t.TempDir()
		logfile := filepath.Join(badCfgDir, "logdir")
		require.NoError(t, os.WriteFile(logfile, []byte{1, 2, 3}, os.ModePerm))
		cfg, err := config.LoadFile(filepath.Join(goodCfg, "protocol.privnet.yml"))
		require.NoError(t, err, "could not load config")
		cfg.ApplicationConfiguration.LogPath = filepath.Join(logfile, "file.log")
		out, err := yaml.Marshal(cfg)
		require.NoError(t, err)

		badCfgPath := filepath.Join(badCfgDir, "protocol.privnet.yml")
		require.NoError(t, os.WriteFile(badCfgPath, out, os.ModePerm))

		*cfgPath = badCfgDir
		require.Error(t, restoreDB(ctx))

		*cfgPath = goodCfg
	})
	t.Run("invalid bc config", func(t *testing.T) {
		badCfgDir := t.TempDir()
		cfg, err := config.LoadFile(filepath.Join(goodCfg, "protocol.privnet.yml"))
		require.NoError(t, err, "could not load config")
		cfg.ApplicationConfiguration.DBConfiguration.Type = ""
		out, err := yaml.Marshal(cfg)
		require.NoError(t, err)

		badCfgPath := filepath.Join(badCfgDir, "protocol.privnet.yml")
		require.NoError(t, os.WriteFile(badCfgPath, out, os.ModePerm))

		*cfgPath = badCfgDir
		require.Error(t, restoreDB(ctx))

		*cfgPath = goodCfg
	})

	in := set.String("in", testDump, "")
	incremental := set.Bool("incremental", false, "")
	t.Run("invalid in", func(t *testing.T) {
		*in = "unknown-file"
		require.Error(t, restoreDB(ctx))

		*in = testDump
	})
	t.Run("corrupted in: invalid block count", func(t *testing.T) {
		inPath := filepath.Join(t.TempDir(), "file3.acc")
		require.NoError(t, os.WriteFile(inPath, []byte{1, 2, 3}, // file is expected to start from uint32
			os.ModePerm))
		*in = inPath
		require.Error(t, restoreDB(ctx))

		*in = testDump
	})
	t.Run("corrupted in: corrupted block", func(t *testing.T) {
		inPath := filepath.Join(t.TempDir(), "file3.acc")
		b, err := os.ReadFile(testDump)
		require.NoError(t, err)
		b[5] = 0xff // file is expected to start from uint32 (4 bytes) followed by the first block, so corrupt the first block bytes
		require.NoError(t, os.WriteFile(inPath, b, os.ModePerm))
		*in = inPath
		require.Error(t, restoreDB(ctx))

		*in = testDump
	})
	t.Run("incremental dump", func(t *testing.T) {
		inPath := filepath.Join(t.TempDir(), "file1_incremental.acc")
		b, err := os.ReadFile(testDump)
		require.NoError(t, err)
		start := make([]byte, 4)
		t.Run("good", func(t *testing.T) {
			binary.LittleEndian.PutUint32(start, 1) // start from the first block
			require.NoError(t, os.WriteFile(inPath, append(start, b...),
				os.ModePerm))
			*in = inPath
			*incremental = true

			require.NoError(t, restoreDB(ctx))
		})
		t.Run("dump is too high", func(t *testing.T) {
			binary.LittleEndian.PutUint32(start, 2) // start from the second block
			require.NoError(t, os.WriteFile(inPath, append(start, b...),
				os.ModePerm))
			*in = inPath
			*incremental = true

			require.Error(t, restoreDB(ctx))
		})

		*in = testDump
		*incremental = false
	})

	set.String("dump", saveDump, "")
	require.NoError(t, restoreDB(ctx))
}

func TestConfigureAddresses(t *testing.T) {
	defaultAddress := "http://localhost:10333"
	customAddress := "http://localhost:10334"

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
			RPC: config.RPC{
				BasicService: config.BasicService{
					Address: customAddress,
				},
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
			Pprof: config.BasicService{
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
			Prometheus: config.BasicService{
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
		_, _, err := initBlockChain(config.Config{}, nil)
		require.Error(t, err)
	})

	t.Run("empty logger", func(t *testing.T) {
		_, _, err := initBlockChain(config.Config{
			ApplicationConfiguration: config.ApplicationConfiguration{
				DBConfiguration: dbconfig.DBConfiguration{
					Type: dbconfig.InMemoryDB,
				},
			},
		}, nil)
		require.Error(t, err)
	})
}

func TestResetDB(t *testing.T) {
	d := t.TempDir()
	err := os.Chdir(d)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(serverTestWD)) })
	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	set.String("config-path", filepath.Join(serverTestWD, "..", "..", "config"), "")
	set.Bool("privnet", true, "")
	set.Bool("debug", true, "")
	set.Int("height", 0, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	err = resetDB(ctx)
	require.NoError(t, err)
}
