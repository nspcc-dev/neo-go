package vm

import (
	"math/big"
	"testing"

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
