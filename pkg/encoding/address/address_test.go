package address

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint160DecodeEncodeAddress(t *testing.T) {
	addrs := []string{
		"NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR",
		"NPCD6gAxNuuJqssZY1eCJabuaz4BjBUHab",
		"NUJUhgvvQyp6AmDBg3QRQ1cmRkMRhaXqZP",
	}
	for _, addr := range addrs {
		val, err := StringToUint160(addr)
		require.NoError(t, err)
		assert.Equal(t, addr, Uint160ToString(val))
	}
}

func TestUint160DecodeKnownAddress(t *testing.T) {
	address := "NNnFn8iHWWnJe9QYoN1r4PeXMuVpfLVRS7"

	val, err := StringToUint160(address)
	require.NoError(t, err)

	assert.Equal(t, "b28427088a3729b2536d10122960394e8be6721f", val.StringLE())
	assert.Equal(t, "1f72e68b4e39602912106d53b229378a082784b2", val.String())
}

func TestUint160DecodeBadBase58(t *testing.T) {
	address := "AJeAEsmeD6t279Dx4n2HWdUvUmmXQ4iJv@"

	_, err := StringToUint160(address)
	require.Error(t, err)
}

func TestUint160DecodeBadPrefix(t *testing.T) {
	// The same AJeAEsmeD6t279Dx4n2HWdUvUmmXQ4iJvP key encoded with 0x18 prefix.
	address := "AhymDz4vvHLtvaN36CMbzkki7H2U8ENb8F"

	_, err := StringToUint160(address)
	require.Error(t, err)
}

func TestPrefixFirstLetter(t *testing.T) {
	u := util.Uint160{}
	require.EqualValues(t, 'N', Uint160ToString(u)[0])

	for i := range u {
		u[i] = 0xFF
	}
	require.EqualValues(t, 'N', Uint160ToString(u)[0])
}
