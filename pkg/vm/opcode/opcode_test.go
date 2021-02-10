package opcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestFromString(t *testing.T) {
	_, err := FromString("abcdef")
	require.Error(t, err)

	op, err := FromString(MUL.String())
	require.NoError(t, err)
	require.Equal(t, MUL, op)
}

func TestIsValid(t *testing.T) {
	require.True(t, IsValid(ADD))
	require.True(t, IsValid(CONVERT))
	require.False(t, IsValid(0xff))
	require.False(t, IsValid(0xa5))
}
