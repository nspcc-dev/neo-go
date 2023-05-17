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
	v.loadScriptWithCallingHash(prog, nil, util.Uint160{}, util.Uint160{}, callflag.All, 1, 3, nil)
	require.Equal(t, []int{}, v.Context().BreakPoints())
}
