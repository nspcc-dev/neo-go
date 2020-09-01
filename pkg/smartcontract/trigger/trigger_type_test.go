package trigger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringer(t *testing.T) {
	tests := map[Type]string{
		System:       "System",
		Application:  "Application",
		Verification: "Verification",
	}
	for o, s := range tests {
		assert.Equal(t, s, o.String())
	}
}

func TestEncodeBynary(t *testing.T) {
	tests := map[Type]byte{
		System:       0x01,
		Verification: 0x20,
		Application:  0x40,
	}
	for o, b := range tests {
		assert.Equal(t, b, byte(o))
	}
}

func TestDecodeBynary(t *testing.T) {
	tests := map[Type]byte{
		System:       0x01,
		Verification: 0x20,
		Application:  0x40,
	}
	for o, b := range tests {
		assert.Equal(t, o, Type(b))
	}
}
