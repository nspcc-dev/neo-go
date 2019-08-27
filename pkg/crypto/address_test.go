package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint160DecodeEncodeAddress(t *testing.T) {
	addrs := []string{
		"AMLr1CpPQtbEdiJdriX1HpRNMZUwbU2Huj",
		"AKtwd3DRXj3nL5kHMUoNsdnsCEVjnuuTFF",
		"AMxkaxFVG8Q1BhnB4fjTA5ZmUTEnnTMJMa",
	}
	for _, addr := range addrs {
		val, err := Uint160DecodeAddress(addr)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, addr, AddressFromUint160(val))
	}
}

func TestUint160DecodeKnownAddress(t *testing.T) {
	address := "AJeAEsmeD6t279Dx4n2HWdUvUmmXQ4iJvP"

	val, err := Uint160DecodeAddress(address)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "b28427088a3729b2536d10122960394e8be6721f", val.ReverseString())
	assert.Equal(t, "1f72e68b4e39602912106d53b229378a082784b2", val.String())
}
