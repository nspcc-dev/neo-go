package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextInstruction(t *testing.T) {
	// PUSHBYTES1 2
	builder := NewBuilder()
	builder.EmitBytes([]byte{0x02}) //[]byte{0x01, 0x02}

	ctx := NewContext(builder.Bytes())
	op := ctx.Next()
	byt := ctx.readByte()

	assert.Equal(t, PUSHBYTES1, op)
	assert.Equal(t, byte(2), byt)
}
