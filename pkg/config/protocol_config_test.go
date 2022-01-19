package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtocolConfigurationValidation(t *testing.T) {
	p := &ProtocolConfiguration{
		ValidatorsCount: 1,
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		NativeUpdateHistories: map[string][]uint32{
			"someContract": []uint32{0, 10},
		},
	}
	require.Error(t, p.Validate())
}
