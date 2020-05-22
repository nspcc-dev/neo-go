package trigger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringer(t *testing.T) {
	tests := map[Type]string{
		Application:   "Application",
		ApplicationR:  "ApplicationR",
		Verification:  "Verification",
		VerificationR: "VerificationR",
	}
	for o, s := range tests {
		assert.Equal(t, s, o.String())
	}
}

func TestEncodeBynary(t *testing.T) {
	tests := map[Type]byte{
		Verification:  0x00,
		VerificationR: 0x01,
		Application:   0x10,
		ApplicationR:  0x11,
	}
	for o, b := range tests {
		assert.Equal(t, b, byte(o))
	}
}

func TestDecodeBynary(t *testing.T) {
	tests := map[Type]byte{
		Verification:  0x00,
		VerificationR: 0x01,
		Application:   0x10,
		ApplicationR:  0x11,
	}
	for o, b := range tests {
		assert.Equal(t, o, Type(b))
	}
}
