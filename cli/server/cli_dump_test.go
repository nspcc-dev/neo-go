package server_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// generated via `go run ./scripts/gendump/main.go --out ./cli/server/testdata/chain50x2.acc --blocks 50 --txs 2`.
const inDump = "./testdata/chain50x2.acc"

func TestDBRestoreDump(t *testing.T) {
	tmpDir := t.TempDir()

	loadConfig := func(t *testing.T) config.Config {
		chainPath := filepath.Join(tmpDir, "neogotestchain")
		cfg, err := config.LoadFile(filepath.Join("..", "..", "config", "protocol.unit_testnet.yml"))
		require.NoError(t, err, "could not load config")
		cfg.ApplicationConfiguration.DBConfiguration.Type = dbconfig.LevelDB
		cfg.ApplicationConfiguration.DBConfiguration.LevelDBOptions.DataDirectoryPath = chainPath
		return cfg
	}

	cfg := loadConfig(t)
	out, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	cfgPath := filepath.Join(tmpDir, "protocol.unit_testnet.yml")
	require.NoError(t, os.WriteFile(cfgPath, out, os.ModePerm))

	e := testcli.NewExecutor(t, false)

	stateDump := filepath.Join(tmpDir, "neogo.teststate")
	baseArgs := []string{"neo-go", "db", "restore", "--unittest",
		"--config-path", tmpDir, "--in", inDump, "--dump", stateDump}

	t.Run("excessive restore parameters", func(t *testing.T) {
		e.RunWithError(t, append(baseArgs, "something")...)
	})
	// First 15 blocks.
	e.Run(t, append(baseArgs, "--count", "15")...)

	// Big count.
	e.RunWithError(t, append(baseArgs, "--count", "1000")...)

	// Continue 15..25
	e.Run(t, append(baseArgs, "--count", "10")...)

	// Continue till end.
	e.Run(t, baseArgs...)

	// Dump and compare.
	dumpPath := filepath.Join(tmpDir, "testdump.acc")

	t.Run("missing config", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "db", "dump", "--privnet",
			"--config-path", tmpDir, "--out", dumpPath)
	})
	t.Run("bad logger config", func(t *testing.T) {
		badConfigDir := t.TempDir()
		logfile := filepath.Join(badConfigDir, "logdir")
		require.NoError(t, os.WriteFile(logfile, []byte{1, 2, 3}, os.ModePerm))
		cfg = loadConfig(t)
		cfg.ApplicationConfiguration.LogPath = filepath.Join(logfile, "file.log")
		out, err = yaml.Marshal(cfg)
		require.NoError(t, err)

		cfgPath = filepath.Join(badConfigDir, "protocol.unit_testnet.yml")
		require.NoError(t, os.WriteFile(cfgPath, out, os.ModePerm))

		e.RunWithError(t, "neo-go", "db", "dump", "--unittest",
			"--config-path", badConfigDir, "--out", dumpPath)
	})
	t.Run("bad storage config", func(t *testing.T) {
		badConfigDir := t.TempDir()
		logfile := filepath.Join(badConfigDir, "logdir")
		require.NoError(t, os.WriteFile(logfile, []byte{1, 2, 3}, os.ModePerm))
		cfg = loadConfig(t)
		cfg.ApplicationConfiguration.DBConfiguration.Type = ""
		out, err = yaml.Marshal(cfg)
		require.NoError(t, err)

		cfgPath = filepath.Join(badConfigDir, "protocol.unit_testnet.yml")
		require.NoError(t, os.WriteFile(cfgPath, out, os.ModePerm))

		e.RunWithError(t, "neo-go", "db", "dump", "--unittest",
			"--config-path", badConfigDir, "--out", dumpPath)
	})

	baseCmd := []string{"neo-go", "db", "dump", "--unittest",
		"--config-path", tmpDir, "--out", dumpPath}

	t.Run("invalid start/count", func(t *testing.T) {
		e.RunWithError(t, append(baseCmd, "--start", "5", "--count", strconv.Itoa(50-5+1+1))...)
	})
	t.Run("excessive dump parameters", func(t *testing.T) {
		e.RunWithError(t, append(baseCmd, "something")...)
	})

	e.Run(t, baseCmd...)

	d1, err := os.ReadFile(inDump)
	require.NoError(t, err)
	d2, err := os.ReadFile(dumpPath)
	require.NoError(t, err)
	require.Equal(t, d1, d2, "dumps differ")
}

func TestDBDumpRestoreIncremental(t *testing.T) {
	tmpDir := t.TempDir()
	chainPath := filepath.Join(tmpDir, "neogotestchain")
	nonincDump := filepath.Join(tmpDir, "nonincDump.acc")
	incDump := filepath.Join(tmpDir, "incDump.acc")

	cfg, err := config.LoadFile(filepath.Join("..", "..", "config", "protocol.unit_testnet.yml"))
	require.NoError(t, err, "could not load config")
	cfg.ApplicationConfiguration.DBConfiguration.Type = dbconfig.LevelDB
	cfg.ApplicationConfiguration.DBConfiguration.LevelDBOptions.DataDirectoryPath = chainPath
	out, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	cfgPath := filepath.Join(tmpDir, "protocol.unit_testnet.yml")
	require.NoError(t, os.WriteFile(cfgPath, out, os.ModePerm))

	e := testcli.NewExecutor(t, false)

	// Create DB from dump.
	e.Run(t, "neo-go", "db", "restore", "--unittest", "--config-path", tmpDir, "--in", inDump)

	// Create two dumps: non-incremental and incremental.
	dumpBaseArgs := []string{"neo-go", "db", "dump", "--unittest",
		"--config-path", tmpDir}

	// Dump first 15 blocks to a non-incremental dump.
	e.Run(t, append(dumpBaseArgs, "--out", nonincDump, "--count", "15")...)

	// Dump second 15 blocks to an incremental dump.
	e.Run(t, append(dumpBaseArgs, "--out", incDump, "--start", "15", "--count", "15")...)

	// Clean the DB.
	require.NoError(t, os.RemoveAll(chainPath))

	// Restore chain from two dumps.
	restoreBaseArgs := []string{"neo-go", "db", "restore", "--unittest", "--config-path", tmpDir}

	// Restore first 15 blocks from non-incremental dump.
	e.Run(t, append(restoreBaseArgs, "--in", nonincDump)...)

	// Restore second 15 blocks from incremental dump.
	e.Run(t, append(restoreBaseArgs, "--in", incDump, "-n", "--count", "15")...)
}
