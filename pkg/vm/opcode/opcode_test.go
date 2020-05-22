package opcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Nothing more to test here, really.
func TestStringer(t *testing.T) {
	tests := map[Opcode]string{
		ADD:    "ADD",
		SUB:    "SUB",
		ASSERT: "ASSERT",
		0xff:   "Opcode(255)",
	}
	for o, s := range tests {
		assert.Equal(t, s, o.String())
	}
}
