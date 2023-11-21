package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/stretchr/testify/require"
)

const testConfigPath = "./testdata/protocol.test.yml"

func TestUnexpectedNativeUpdateHistoryContract(t *testing.T) {
	_, err := LoadFile(testConfigPath)
	require.Error(t, err)
}

func TestUnknownConfigFields(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "protocol.testnet.yml")
	require.NoError(t, os.WriteFile(cfg, []byte(`UnknownConfigurationField: 123`), os.ModePerm))

	t.Run("LoadFile", func(t *testing.T) {
		_, err := LoadFile(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "field UnknownConfigurationField not found in type config.Config")
	})
	t.Run("Load", func(t *testing.T) {
		_, err := Load(tmp, netmode.TestNet)
		require.Error(t, err)
		require.Contains(t, err.Error(), "field UnknownConfigurationField not found in type config.Config")
	})
}
