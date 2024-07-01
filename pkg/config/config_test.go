package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestLoadFileWithMissingDefaultConfigPath(t *testing.T) {
	var cfgPrivNet Config
	cfg, err := LoadFile(fmt.Sprintf("%s/protocol.%s.yml", DefaultConfigPath, netmode.PrivNet))
	require.Nil(t, err)
	decoder := yaml.NewDecoder(bytes.NewReader(config.PrivNet))
	err = decoder.Decode(&cfgPrivNet)
	require.NoError(t, err)
	require.Equal(t, cfg, cfgPrivNet)

	_, err = LoadFile(fmt.Sprintf("%s/protocol.%s.yml", os.TempDir(), netmode.PrivNet))
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesn't exist and no matching embedded config was found")

	_, err = LoadFile(fmt.Sprintf("%s/protocol.%s.yml", DefaultConfigPath, "aaa"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesn't exist and no matching embedded config was found")
}
