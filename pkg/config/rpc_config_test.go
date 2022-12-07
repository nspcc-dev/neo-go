package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestRPC_UnmarshalBasicService is aimed to check that BasicService config of
// RPC service can be properly unmarshalled. This test may be removed after
// Address and Port config fields removal.
func TestRPC_UnmarshalBasicService(t *testing.T) {
	data := `
Enabled: true
Port: 10332
MaxGasInvoke: 15
`
	cfg := &RPC{}
	err := yaml.Unmarshal([]byte(data), &cfg)
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, uint16(10332), *cfg.Port)
}
