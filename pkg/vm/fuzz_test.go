//go:build go1.18

package vm

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

var fuzzSeedValidScripts = [][]byte{
	makeProgram(opcode.PUSH1, opcode.PUSH10, opcode.ADD),
	makeProgram(opcode.PUSH10, opcode.JMP, 3, opcode.ABORT, opcode.RET),
	makeProgram(opcode.PUSHINT16, 1, 2, opcode.PUSHINT32, 3, 4, opcode.DROP),
	makeProgram(opcode.PUSH2, opcode.NEWARRAY, opcode.DUP, opcode.PUSH0, opcode.PUSH1, opcode.SETITEM, opcode.VALUES),
	append([]byte{byte(opcode.PUSHDATA1), 10}, randomBytes(10)...),
	append([]byte{byte(opcode.PUSHDATA1), 100}, randomBytes(100)...),
	// Simplified version of fuzzer output from #2659.
	{byte(opcode.CALL), 3, byte(opcode.ASSERT),
		byte(opcode.CALL), 3, byte(opcode.ASSERT),
		byte(opcode.DEPTH), byte(opcode.PACKSTRUCT), byte(opcode.DUP),
		byte(opcode.UNPACK), byte(opcode.PACKSTRUCT), byte(opcode.POPITEM),
		byte(opcode.DEPTH)},
}

func FuzzIsScriptCorrect(f *testing.F) {
	for _, s := range fuzzSeedValidScripts {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, script []byte) {
		require.NotPanics(t, func() {
			_ = IsScriptCorrect(script, nil)
		})
	})
}

func FuzzVMDontPanic(f *testing.F) {
	for _, s := range fuzzSeedValidScripts {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, script []byte) {
		if IsScriptCorrect(script, nil) != nil {
			return
		}

		v := load(script)

		// Prevent infinite loops from being reported as fail.
		v.GasLimit = 1000
		v.getPrice = func(opcode.Opcode, []byte) int64 {
			return 1
		}

		require.NotPanics(t, func() {
			_ = v.Run()
		})
	})
}
