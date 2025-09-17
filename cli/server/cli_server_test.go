package server_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/cli/server"
	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestServerStart(t *testing.T) {
	tmpDir := t.TempDir()
	goodCfg, err := config.LoadFile(filepath.Join("..", "..", "config", "protocol.unit_testnet.yml"))
	require.NoError(t, err, "could not load config")
	ptr := &goodCfg
	saveCfg := func(t *testing.T, f func(cfg *config.Config)) string {
		cfg := *ptr
		chainPath := filepath.Join(t.TempDir(), "neogotestchain")
		cfg.ApplicationConfiguration.DBConfiguration.Type = dbconfig.LevelDB
		cfg.ApplicationConfiguration.DBConfiguration.LevelDBOptions.DataDirectoryPath = chainPath
		f(&cfg)
		out, err := yaml.Marshal(cfg)
		require.NoError(t, err)

		cfgPath := filepath.Join(tmpDir, "protocol.unit_testnet.yml")
		require.NoError(t, os.WriteFile(cfgPath, out, os.ModePerm))
		t.Cleanup(func() {
			require.NoError(t, os.Remove(cfgPath))
		})
		return cfgPath
	}

	baseCmd := []string{"neo-go", "node", "--unittest", "--config-path", tmpDir}
	e := testcli.NewExecutor(t, false)

	t.Run("invalid config path", func(t *testing.T) {
		e.RunWithError(t, baseCmd...)
	})
	t.Run("bad logger config", func(t *testing.T) {
		badConfigDir := t.TempDir()
		logfile := filepath.Join(badConfigDir, "logdir")
		require.NoError(t, os.WriteFile(logfile, []byte{1, 2, 3}, os.ModePerm))
		saveCfg(t, func(cfg *config.Config) {
			cfg.ApplicationConfiguration.LogPath = filepath.Join(logfile, "file.log")
		})
		e.RunWithError(t, baseCmd...)
	})
	t.Run("invalid storage", func(t *testing.T) {
		saveCfg(t, func(cfg *config.Config) {
			cfg.ApplicationConfiguration.DBConfiguration.Type = ""
		})
		e.RunWithError(t, baseCmd...)
	})
	t.Run("stateroot service is on && StateRootInHeader=true", func(t *testing.T) {
		saveCfg(t, func(cfg *config.Config) {
			cfg.ApplicationConfiguration.StateRoot.Enabled = true
			cfg.ProtocolConfiguration.StateRootInHeader = true
		})
		e.RunWithError(t, baseCmd...)
	})
	t.Run("invalid Oracle config", func(t *testing.T) {
		saveCfg(t, func(cfg *config.Config) {
			cfg.ApplicationConfiguration.Oracle.Enabled = true
			cfg.ApplicationConfiguration.Oracle.UnlockWallet.Path = "bad_orc_wallet.json"
		})
		e.RunWithError(t, baseCmd...)
	})
	t.Run("invalid consensus config", func(t *testing.T) {
		saveCfg(t, func(cfg *config.Config) {
			cfg.ApplicationConfiguration.Consensus.Enabled = true
			cfg.ApplicationConfiguration.Consensus.UnlockWallet.Path = "bad_consensus_wallet.json"
		})
		e.RunWithError(t, baseCmd...)
	})
	t.Run("invalid Notary config", func(t *testing.T) {
		t.Run("malformed config", func(t *testing.T) {
			saveCfg(t, func(cfg *config.Config) {
				cfg.ProtocolConfiguration.P2PSigExtensions = false
				cfg.ApplicationConfiguration.P2PNotary.Enabled = true
			})
			e.RunWithError(t, baseCmd...)
		})
		t.Run("invalid wallet", func(t *testing.T) {
			saveCfg(t, func(cfg *config.Config) {
				cfg.ProtocolConfiguration.P2PSigExtensions = true
				cfg.ApplicationConfiguration.P2PNotary.Enabled = true
				cfg.ApplicationConfiguration.P2PNotary.UnlockWallet.Path = "bad_notary_wallet.json"
			})
			e.RunWithError(t, baseCmd...)
		})
	})
	// We can't properly shutdown server on windows and release the resources.
	// Also, windows doesn't support SIGHUP and SIGINT.
	if runtime.GOOS != "windows" {
		saveCfg(t, func(cfg *config.Config) {})
		t.Run("excessive parameters", func(t *testing.T) {
			e.RunWithError(t, append(baseCmd, "something")...)
		})
		t.Run("good", func(t *testing.T) {
			go func() {
				e.Run(t, baseCmd...)
			}()

			var line string
			require.Eventually(t, func() bool {
				line, err = e.Out.ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					t.Fatalf("unexpected error while reading CLI output: %s", err)
				}
				return err == nil
			}, 2*time.Second, 100*time.Millisecond)
			for expected := range strings.SplitSeq(server.Logo(), "\n") {
				// It should be regexp, so escape all backslashes.
				expected = strings.ReplaceAll(expected, `\`, `\\`)
				e.CheckLine(t, line, expected)
				line = e.GetNextLine(t)
			}
			e.CheckNextLine(t, "")
			e.CheckEOF(t)
		})
	}
}
