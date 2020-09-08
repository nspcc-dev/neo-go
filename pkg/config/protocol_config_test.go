package config

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestGetFreeGas(t *testing.T) {
	fixed10 := util.Fixed8FromInt64(10)
	fixed50 := util.Fixed8FromInt64(50)
	p := ProtocolConfiguration{
		FreeGasLimit: map[uint32]util.Fixed8{
			0:       fixed10,
			6200000: fixed50,
		},
	}
	require.Equal(t, fixed10, p.GetFreeGas(0))
	require.Equal(t, fixed10, p.GetFreeGas(1000))
	require.Equal(t, fixed10, p.GetFreeGas(1000000))
	require.Equal(t, fixed10, p.GetFreeGas(6100000))
	require.Equal(t, fixed50, p.GetFreeGas(6200000))
	require.Equal(t, fixed50, p.GetFreeGas(7000000))
}
