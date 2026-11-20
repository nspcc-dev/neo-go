package vm

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestVM_Debug(t *testing.T) {
	prog := makeProgram(opcode.CALL, 3, opcode.RET,
		opcode.PUSH2, opcode.PUSH3, opcode.ADD, opcode.RET)
	t.Run("BreakPoint", func(t *testing.T) {
		v := load(prog)
		v.AddBreakPoint(3)
		v.AddBreakPoint(5)
		require.NoError(t, v.Run())
		require.Equal(t, 3, v.Context().NextIP())
		require.NoError(t, v.Run())
		require.Equal(t, 5, v.Context().NextIP())
		require.NoError(t, v.Run())
		require.Equal(t, 1, v.estack.Len())
		require.Equal(t, big.NewInt(5), v.estack.Top().Value())
	})
	t.Run("StepInto", func(t *testing.T) {
		v := load(prog)
		require.NoError(t, v.StepInto())
		require.Equal(t, 3, v.Context().NextIP())
		require.NoError(t, v.StepOut())
		require.Equal(t, 2, v.Context().NextIP())
		require.Equal(t, 1, v.estack.Len())
		require.Equal(t, big.NewInt(5), v.estack.Top().Value())
	})
	t.Run("StepOver", func(t *testing.T) {
		v := load(prog)
		require.NoError(t, v.StepOver())
		require.Equal(t, 2, v.Context().NextIP())
		require.Equal(t, 1, v.estack.Len())
		require.Equal(t, big.NewInt(5), v.estack.Top().Value())
	})
}

func TestVM_StepInstructions(t *testing.T) {
	// Script under test:
	//   0: INITSLOT 1,0
	//   3: PUSH10
	//   4: STLOC0
	//   5: JMP +6      -> 11
	//   7: LDLOC0
	//   8: PUSH1
	//   9: SUB
	//  10: STLOC0
	//  11: LDLOC0
	//  12: PUSH0
	//  13: GT
	//  14: JMPIF -7    -> 11
	//  16: LDLOC0
	//  17: RET
	prog := makeProgram(opcode.INITSLOT, 1, 0, opcode.PUSH10, opcode.STLOC0,
		opcode.JMP, 6,
		opcode.LDLOC0, opcode.PUSH1, opcode.SUB, opcode.STLOC0,
		opcode.LDLOC0, opcode.PUSH0, opcode.GT, opcode.JMPIF, 0xf9,
		opcode.LDLOC0)

	t.Run("variable length instruction", func(t *testing.T) {
		v := load(prog)
		require.NoError(t, v.StepInstructions(1))
		require.Equal(t, 3, v.Context().NextIP()) // INITSLOT occupies bytes 0..2
		require.True(t, v.AtBreakpoint())
	})
	t.Run("counts executed instructions", func(t *testing.T) {
		v := load(prog)
		require.NoError(t, v.StepInstructions(3))
		require.Equal(t, 5, v.Context().NextIP()) // INITSLOT, PUSH10, STLOC0
		require.NoError(t, v.StepInstructions(3))
		require.Equal(t, 13, v.Context().NextIP()) // JMP, LDLOC0, PUSH0
	})
	t.Run("jump is followed", func(t *testing.T) {
		v := load(prog)
		require.NoError(t, v.StepInstructions(4))
		require.Equal(t, 11, v.Context().NextIP()) // the JMP skips bytes 7..10
		require.True(t, v.AtBreakpoint())
	})
	t.Run("breakpoint stops the loop", func(t *testing.T) {
		v := load(prog)
		v.AddBreakPoint(11)
		require.NoError(t, v.StepInstructions(10))
		require.Equal(t, 11, v.Context().NextIP())
		require.True(t, v.AtBreakpoint())
	})
	t.Run("stopped VM is not stepped", func(t *testing.T) {
		v := load(prog)
		require.NoError(t, v.Run())
		require.True(t, v.HasHalted())
		require.NoError(t, v.StepInstructions(3))
		require.True(t, v.HasHalted())
		require.Nil(t, v.Context())
	})
}

func TestContext_BreakPoints(t *testing.T) {
	prog := makeProgram(opcode.CALL, 3, opcode.RET,
		opcode.PUSH2, opcode.PUSH3, opcode.ADD, opcode.RET)
	v := load(prog)
	v.AddBreakPoint(3)
	v.AddBreakPoint(5)
	require.Equal(t, []int{3, 5}, v.Context().BreakPoints())

	// Preserve the set of breakpoints on Call.
	v.Call(3)
	require.Equal(t, []int{3, 5}, v.Context().BreakPoints())

	// New context -> clean breakpoints.
	v.loadScriptWithCallingHash(prog, nil, nil, util.Uint160{}, util.Uint160{}, callflag.All, 1, 3, nil, nil, false)
	require.Nil(t, v.Context().BreakPoints())

	v.AddBreakPoint(3)
	v.AddBreakPoint(3)
	v.AddBreakPoint(5)
	require.Equal(t, []int{3, 3, 5}, v.Context().BreakPoints())
	v.RemoveBreakPoint(3)
	require.Equal(t, []int{5}, v.Context().BreakPoints())
}
