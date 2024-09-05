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

func TestDumpBin(t *testing.T) {
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

	restoreArgs := []string{"neo-go", "db", "restore",
		"--config-file", cfgPath, "--in", inDump}
	e.Run(t, restoreArgs...)

	t.Run("missing output directory", func(t *testing.T) {
		args := []string{"neo-go", "db", "dump-bin",
			"--config-file", cfgPath, "--out", ""}
		e.RunWithErrorCheck(t, "output directory is not specified", args...)
	})

	t.Run("successful dump", func(t *testing.T) {
		outDir := filepath.Join(tmpDir, "blocks")
		args := []string{"neo-go", "db", "dump-bin",
			"--config-file", cfgPath, "--out", outDir, "--count", "5", "--start", "0"}

		e.Run(t, args...)

		require.DirExists(t, outDir)

		for i := range 5 {
			blockFile := filepath.Join(outDir, "block-"+strconv.Itoa(i)+".bin")
			require.FileExists(t, blockFile)
		}
	})

	t.Run("invalid block range", func(t *testing.T) {
		outDir := filepath.Join(tmpDir, "invalid-blocks")
		args := []string{"neo-go", "db", "dump-bin",
			"--config-file", cfgPath, "--out", outDir, "--count", "1000", "--start", "0"}

		e.RunWithError(t, args...)
	})

	t.Run("zero blocks (full chain dump)", func(t *testing.T) {
		outDir := filepath.Join(tmpDir, "full-dump")
		args := []string{"neo-go", "db", "dump-bin",
			"--config-file", cfgPath, "--out", outDir}

		e.Run(t, args...)

		require.DirExists(t, outDir)
		for i := range 5 {
			blockFile := filepath.Join(outDir, "block-"+strconv.Itoa(i)+".bin")
			require.FileExists(t, blockFile)
		}
	})

	t.Run("invalid config file", func(t *testing.T) {
		outDir := filepath.Join(tmpDir, "blocks")
		args := []string{"neo-go", "db", "dump-bin",
			"--config-file", "invalid-config-path", "--out", outDir}

		e.RunWithError(t, args...)
	})
}
