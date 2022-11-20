package vm

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/invocations"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestInvocationTree(t *testing.T) {
	script := []byte{
		byte(opcode.PUSH3), byte(opcode.DEC),
		byte(opcode.DUP), byte(opcode.PUSH0), byte(opcode.JMPEQ), (2 + 2 + 2 + 6 + 1),
		byte(opcode.CALL), (2 + 2), // CALL shouldn't affect invocation tree.
		byte(opcode.JMP), 0xf9, // DEC
		byte(opcode.SYSCALL), 0, 0, 0, 0, byte(opcode.DROP),
		byte(opcode.RET),
		byte(opcode.RET),
		byte(opcode.PUSHINT8), 0xff,
	}

	cnt := 0
	v := newTestVM()
	v.SyscallHandler = func(v *VM, _ uint32) error {
		if len(v.Istack()) > 4 { // top -> call -> syscall -> call -> syscall -> ...
			v.Estack().PushVal(1)
			return nil
		}
		cnt++
		v.LoadScriptWithHash(script, util.Uint160{byte(cnt)}, 0)
		return nil
	}
	v.EnableInvocationTree()
	v.LoadScript(script)
	topHash := v.Context().ScriptHash()
	require.NoError(t, v.Run())

	res := &invocations.Tree{
		Calls: []*invocations.Tree{{
			Current: topHash,
			Calls: []*invocations.Tree{
				{
					Current: util.Uint160{1},
					Calls: []*invocations.Tree{
						{
							Current: util.Uint160{2},
						},
						{
							Current: util.Uint160{3},
						},
					},
				},
				{
					Current: util.Uint160{4},
					Calls: []*invocations.Tree{
						{
							Current: util.Uint160{5},
						},
						{
							Current: util.Uint160{6},
						},
					},
				},
			},
		}},
	}
	require.Equal(t, res, v.GetInvocationTree())
}
