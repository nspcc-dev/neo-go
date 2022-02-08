package main

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/cli/server"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestServerStart(t *testing.T) {
	tmpDir := t.TempDir()
	goodCfg, err := config.LoadFile(filepath.Join("..", "config", "protocol.unit_testnet.yml"))
	require.NoError(t, err, "could not load config")
	ptr := &goodCfg
	saveCfg := func(t *testing.T, f func(cfg *config.Config)) string {
		cfg := *ptr
		chainPath := filepath.Join(t.TempDir(), "neogotestchain")
		cfg.ApplicationConfiguration.DBConfiguration.Type = "leveldb"
		cfg.ApplicationConfiguration.DBConfiguration.LevelDBOptions.DataDirectoryPath = chainPath
		f(&cfg)
		out, err := yaml.Marshal(cfg)
		require.NoError(t, err)

		cfgPath := filepath.Join(tmpDir, "protocol.unit_testnet.yml")
		require.NoError(t, ioutil.WriteFile(cfgPath, out, os.ModePerm))
		t.Cleanup(func() {
			require.NoError(t, os.Remove(cfgPath))
		})
		return cfgPath
	}

	baseCmd := []string{"neo-go", "node", "--unittest", "--config-path", tmpDir}
	e := newExecutor(t, false)

	t.Run("invalid config path", func(t *testing.T) {
		e.RunWithError(t, baseCmd...)
	})
	t.Run("bad logger config", func(t *testing.T) {
		badConfigDir := t.TempDir()
		logfile := filepath.Join(badConfigDir, "logdir")
		require.NoError(t, ioutil.WriteFile(logfile, []byte{1, 2, 3}, os.ModePerm))
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
			cfg.ApplicationConfiguration.UnlockWallet.Path = "bad_consensus_wallet.json"
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
		t.Run("good", func(t *testing.T) {
			saveCfg(t, func(cfg *config.Config) {})

			go func() {
				e.Run(t, baseCmd...)
			}()

			var line string
			require.Eventually(t, func() bool {
				line, err = e.Out.ReadString('\n')
				if err != nil && err != io.EOF {
					t.Fatalf("unexpected error while reading CLI output: %s", err)
				}
				return err == nil
			}, 2*time.Second, 100*time.Millisecond)
			lines := strings.Split(server.Logo(), "\n")
			for _, expected := range lines {
				// It should be regexp, so escape all backslashes.
				expected = strings.ReplaceAll(expected, `\`, `\\`)
				e.checkLine(t, line, expected)
				line = e.getNextLine(t)
			}
			e.checkNextLine(t, "")
			e.checkEOF(t)
		})
	}
}
