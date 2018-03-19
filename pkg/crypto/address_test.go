package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint160DecodeAddress(t *testing.T) {
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
