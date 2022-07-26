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
