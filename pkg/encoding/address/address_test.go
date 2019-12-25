package address

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUint160DecodeEncodeAddress(t *testing.T) {
	addrs := []string{
		"AMLr1CpPQtbEdiJdriX1HpRNMZUwbU2Huj",
		"AKtwd3DRXj3nL5kHMUoNsdnsCEVjnuuTFF",
		"AMxkaxFVG8Q1BhnB4fjTA5ZmUTEnnTMJMa",
	}
	for _, addr := range addrs {
		val, err := DecodeUint160(addr)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, addr, EncodeUint160(val))
	}
}

func TestUint160DecodeKnownAddress(t *testing.T) {
	address := "AJeAEsmeD6t279Dx4n2HWdUvUmmXQ4iJvP"

	val, err := DecodeUint160(address)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "b28427088a3729b2536d10122960394e8be6721f", val.StringLE())
	assert.Equal(t, "1f72e68b4e39602912106d53b229378a082784b2", val.String())
}

func TestUint160DecodeBadBase58(t *testing.T) {
	address := "AJeAEsmeD6t279Dx4n2HWdUvUmmXQ4iJv@"

	_, err := DecodeUint160(address)
	require.Error(t, err)
}

func TestUint160DecodeBadPrefix(t *testing.T) {
	// The same AJeAEsmeD6t279Dx4n2HWdUvUmmXQ4iJvP key encoded with 0x18 prefix.
	address := "AhymDz4vvHLtvaN36CMbzkki7H2U8ENb8F"

	_, err := DecodeUint160(address)
	require.Error(t, err)
}
