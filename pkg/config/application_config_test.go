package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplicationConfigurationEquals(t *testing.T) {
	a := &ApplicationConfiguration{}
	o := &ApplicationConfiguration{}
	require.True(t, a.EqualsButServices(o))
	require.True(t, o.EqualsButServices(a))
	require.True(t, a.EqualsButServices(a))

	cfg1, err := LoadFile(filepath.Join("..", "..", "config", "protocol.mainnet.yml"))
	require.NoError(t, err)
	cfg2, err := LoadFile(filepath.Join("..", "..", "config", "protocol.testnet.yml"))
	require.NoError(t, err)
	require.False(t, cfg1.ApplicationConfiguration.EqualsButServices(&cfg2.ApplicationConfiguration))
}

// TestApplicationConfiguration_UnmarshalRPCBasicService is aimed to check that BasicService
// config of RPC service can be properly unmarshalled.
func TestApplicationConfiguration_UnmarshalRPCBasicService(t *testing.T) {
	cfg, err := LoadFile(filepath.Join("..", "..", "config", "protocol.mainnet.yml"))
	require.NoError(t, err)
	require.True(t, cfg.ApplicationConfiguration.RPC.Enabled)
	require.Equal(t, uint16(10332), cfg.ApplicationConfiguration.RPC.Port)
}
