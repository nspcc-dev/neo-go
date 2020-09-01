package compiler

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func testShortenJumps(t *testing.T, before, after []opcode.Opcode, indices []int) {
	prog := make([]byte, len(before))
	for i := range before {
		prog[i] = byte(before[i])
	}
	raw := shortenJumps(prog, indices)
	actual := make([]opcode.Opcode, len(raw))
	for i := range raw {
		actual[i] = opcode.Opcode(raw[i])
	}
	require.Equal(t, after, actual)
}

func TestShortenJumps(t *testing.T) {
	testCases := map[opcode.Opcode]opcode.Opcode{
		opcode.JMPL:      opcode.JMP,
		opcode.JMPIFL:    opcode.JMPIF,
		opcode.JMPIFNOTL: opcode.JMPIFNOT,
		opcode.JMPEQL:    opcode.JMPEQ,
		opcode.JMPNEL:    opcode.JMPNE,
		opcode.JMPGTL:    opcode.JMPGT,
		opcode.JMPGEL:    opcode.JMPGE,
		opcode.JMPLEL:    opcode.JMPLE,
		opcode.JMPLTL:    opcode.JMPLT,
		opcode.CALLL:     opcode.CALL,
	}
	for op, sop := range testCases {
		t.Run(op.String(), func(t *testing.T) {
			before := []opcode.Opcode{
				op, 6, 0, 0, 0, opcode.PUSH1, opcode.NOP, // <- first jump to here
				op, 9, 12, 0, 0, opcode.PUSH1, opcode.NOP, // <- last jump to here
				op, 255, 0, 0, 0, op, 0xFF - 5, 0xFF, 0xFF, 0xFF,
			}
			after := []opcode.Opcode{
				sop, 3, opcode.PUSH1, opcode.NOP,
				op, 3, 12, 0, 0, opcode.PUSH1, opcode.NOP,
				sop, 249, sop, 0xFF - 2,
			}
			testShortenJumps(t, before, after, []int{0, 14, 19})
		})
	}
	t.Run("NoReplace", func(t *testing.T) {
		b := []byte{0, 1, 2, 3, 4, 5}
		expected := []byte{0, 1, 2, 3, 4, 5}
		require.Equal(t, expected, shortenJumps(b, nil))
	})
	t.Run("InvalidIndex", func(t *testing.T) {
		before := []byte{byte(opcode.PUSH1), 0, 0, 0, 0}
		require.Panics(t, func() {
			shortenJumps(before, []int{0})
		})
	})
	t.Run("SideConditions", func(t *testing.T) {
		t.Run("Forward", func(t *testing.T) {
			before := []opcode.Opcode{
				opcode.JMPL, 5, 0, 0, 0,
				opcode.JMPL, 5, 0, 0, 0,
			}
			after := []opcode.Opcode{
				opcode.JMP, 2,
				opcode.JMP, 2,
			}
			testShortenJumps(t, before, after, []int{0, 5})
		})
		t.Run("Backwards", func(t *testing.T) {
			before := []opcode.Opcode{
				opcode.JMPL, 5, 0, 0, 0,
				opcode.JMPL, 0xFF - 4, 0xFF, 0xFF, 0xFF,
				opcode.JMPL, 0xFF - 4, 0xFF, 0xFF, 0xFF,
			}
			after := []opcode.Opcode{
				opcode.JMPL, 5, 0, 0, 0,
				opcode.JMP, 0xFF - 4,
				opcode.JMP, 0xFF - 1,
			}
			testShortenJumps(t, before, after, []int{5, 10})
		})
	})
}

func TestWriteJumps(t *testing.T) {
	c := new(codegen)
	c.l = []int{10}
	before := []byte{
		byte(opcode.NOP), byte(opcode.JMP), 2, byte(opcode.RET),
		byte(opcode.CALLL), 0, 0, 0, 0, byte(opcode.RET),
		byte(opcode.PUSH2), byte(opcode.RET),
	}
	c.funcs = map[string]*funcScope{
		"init":   {rng: DebugRange{Start: 0, End: 3}},
		"main":   {rng: DebugRange{Start: 4, End: 9}},
		"method": {rng: DebugRange{Start: 10, End: 11}},
	}

	expProg := []byte{
		byte(opcode.NOP), byte(opcode.JMP), 2, byte(opcode.RET),
		byte(opcode.CALL), 3, byte(opcode.RET),
		byte(opcode.PUSH2), byte(opcode.RET),
	}
	expFuncs := map[string]*funcScope{
		"init":   {rng: DebugRange{Start: 0, End: 3}},
		"main":   {rng: DebugRange{Start: 4, End: 6}},
		"method": {rng: DebugRange{Start: 7, End: 8}},
	}

	buf, err := c.writeJumps(before)
	require.NoError(t, err)
	require.Equal(t, expProg, buf)
	require.Equal(t, expFuncs, c.funcs)
}

func TestWriteJumpsLastJump(t *testing.T) {
	c := new(codegen)
	c.l = []int{2}
	prog := []byte{byte(opcode.JMP), 3, byte(opcode.RET), byte(opcode.JMPL), 0, 0, 0, 0}
	expected := []byte{byte(opcode.JMP), 3, byte(opcode.RET), byte(opcode.JMP), 0xFF}
	actual, err := c.writeJumps(prog)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
