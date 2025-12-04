package scparser

import (
	"crypto/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
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

func FuzzParseMultiSigContract(f *testing.F) {
	pubs := make(keys.PublicKeys, 10)
	for i := range pubs {
		p, _ := keys.NewPrivateKey()
		pubs[i] = p.PublicKey()
	}

	s, _ := smartcontract.CreateMultiSigRedeemScript(1, pubs[:1])
	f.Add(s)

	s, _ = smartcontract.CreateMultiSigRedeemScript(3, pubs[:6])
	f.Add(s)

	s, _ = smartcontract.CreateMultiSigRedeemScript(1, pubs)
	f.Add(s)

	f.Fuzz(func(t *testing.T, script []byte) {
		var b [][]byte
		var ok bool
		var n int
		require.NotPanics(t, func() {
			n, b, ok = ParseMultiSigContract(script)
		})
		if ok {
			require.True(t, n <= len(b))
		}
	})
}

func makeProgram(opcodes ...opcode.Opcode) []byte {
	prog := make([]byte, len(opcodes)+1) // RET
	for i := range opcodes {
		prog[i] = byte(opcodes[i])
	}
	prog[len(prog)-1] = byte(opcode.RET)
	return prog
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}
