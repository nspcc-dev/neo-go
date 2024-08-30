package vm

import (
	"encoding/binary"
	"slices"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util/bitfield"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSignatureContract() []byte {
	prog := make([]byte, 40)
	prog[0] = byte(opcode.PUSHDATA1)
	prog[1] = 33
	prog[35] = byte(opcode.SYSCALL)
	binary.LittleEndian.PutUint32(prog[36:], verifyInteropID)
	return prog
}

func TestParseSignatureContract(t *testing.T) {
	prog := testSignatureContract()
	pub := randomBytes(33)
	copy(prog[2:], pub)
	actual, ok := ParseSignatureContract(prog)
	require.True(t, ok)
	require.Equal(t, pub, actual)
}

func TestIsSignatureContract(t *testing.T) {
	t.Run("valid contract", func(t *testing.T) {
		prog := testSignatureContract()
		assert.True(t, IsSignatureContract(prog))
		assert.True(t, IsStandardContract(prog))
	})

	t.Run("invalid interop ID", func(t *testing.T) {
		prog := testSignatureContract()
		binary.LittleEndian.PutUint32(prog[36:], ^verifyInteropID)
		assert.False(t, IsSignatureContract(prog))
		assert.False(t, IsStandardContract(prog))
	})

	t.Run("invalid pubkey size", func(t *testing.T) {
		prog := testSignatureContract()
		prog[1] = 32
		assert.False(t, IsSignatureContract(prog))
		assert.False(t, IsStandardContract(prog))
	})

	t.Run("invalid length", func(t *testing.T) {
		prog := testSignatureContract()
		prog = append(prog, 0)
		assert.False(t, IsSignatureContract(prog))
		assert.False(t, IsStandardContract(prog))
	})
}

func testMultisigContract(t *testing.T, n, m int) []byte {
	pubs := make(keys.PublicKeys, n)
	for i := 0; i < n; i++ {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		pubs[i] = priv.PublicKey()
	}

	prog, err := smartcontract.CreateMultiSigRedeemScript(m, pubs)
	require.NoError(t, err)
	return prog
}

func TestIsMultiSigContract(t *testing.T) {
	t.Run("valid contract", func(t *testing.T) {
		prog := testMultisigContract(t, 2, 2)
		assert.True(t, IsMultiSigContract(prog))
		assert.True(t, IsStandardContract(prog))
	})

	t.Run("0-length", func(t *testing.T) {
		assert.False(t, IsMultiSigContract([]byte{}))
	})

	t.Run("invalid param", func(t *testing.T) {
		prog := []byte{byte(opcode.PUSHDATA1), 10}
		assert.False(t, IsMultiSigContract(prog))
	})

	t.Run("too many keys", func(t *testing.T) {
		prog := testMultisigContract(t, 1025, 1)
		assert.False(t, IsMultiSigContract(prog))
	})

	t.Run("invalid interop ID", func(t *testing.T) {
		prog := testMultisigContract(t, 2, 2)
		prog[len(prog)-4] ^= 0xFF
		assert.False(t, IsMultiSigContract(prog))
	})

	t.Run("invalid keys number", func(t *testing.T) {
		prog := testMultisigContract(t, 2, 2)
		prog[len(prog)-6] = byte(opcode.PUSH3)
		assert.False(t, IsMultiSigContract(prog))
	})

	t.Run("invalid length", func(t *testing.T) {
		prog := testMultisigContract(t, 2, 2)
		prog = append(prog, 0)
		assert.False(t, IsMultiSigContract(prog))
	})
}

func TestIsScriptCorrect(t *testing.T) {
	w := io.NewBufBinWriter()
	emit.String(w.BinWriter, "something")

	jmpOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.JMP, opcode.Opcode(-jmpOff))

	retOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.RET)

	jmplOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.JMPL, opcode.Opcode(0xff), opcode.Opcode(0xff), opcode.Opcode(0xff), opcode.Opcode(0xff))

	tryOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.TRY, opcode.Opcode(3), opcode.Opcode(0xfb)) // -5

	trylOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.TRYL, opcode.Opcode(0xfd), opcode.Opcode(0xff), opcode.Opcode(0xff), opcode.Opcode(0xff),
		opcode.Opcode(9), opcode.Opcode(0), opcode.Opcode(0), opcode.Opcode(0))

	istypeOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.ISTYPE, opcode.Opcode(stackitem.IntegerT))

	pushOff := w.Len()
	emit.String(w.BinWriter, "else")

	good := w.Bytes()

	t.Run("good", func(t *testing.T) {
		require.NoError(t, IsScriptCorrect(good, nil))
	})

	t.Run("bad instruction", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[retOff] = 0xff
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("out of bounds JMP 1", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[jmpOff+1] = 0x80 // -128
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("out of bounds JMP 2", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[jmpOff+1] = 0x7f
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("bad JMP offset 1", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[jmpOff+1] = 0xff // into "something"
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("bad JMP offset 2", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[jmpOff+1] = byte(pushOff - jmpOff + 1)
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("out of bounds JMPL 1", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[jmplOff+1] = byte(-jmplOff - 1)
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("out of bounds JMPL 1", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[jmplOff+1] = byte(len(bad)-jmplOff) + 1
		bad[jmplOff+2] = 0
		bad[jmplOff+3] = 0
		bad[jmplOff+4] = 0
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("JMP to a len(script)", func(t *testing.T) {
		bad := make([]byte, 64) // 64 is the word-size of a bitset.
		bad[0] = byte(opcode.JMP)
		bad[1] = 64
		require.NoError(t, IsScriptCorrect(bad, nil))
	})

	t.Run("bad JMPL offset", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[jmplOff+1] = 0xfe // into JMP
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("out of bounds TRY 1", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[tryOff+1] = byte(-tryOff - 1)
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("out of bounds TRY 2", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[tryOff+1] = byte(len(bad)-tryOff) + 1
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("out of bounds TRY 2", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[tryOff+2] = byte(len(bad)-tryOff) + 1
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("TRY with jumps to a len(script)", func(t *testing.T) {
		bad := make([]byte, 64) // 64 is the word-size of a bitset.
		bad[0] = byte(opcode.TRY)
		bad[1] = 64
		bad[2] = 64
		bad[3] = byte(opcode.RET) // pad so that remaining script (PUSHINT8 0) is even in length.
		require.NoError(t, IsScriptCorrect(bad, nil))
	})

	t.Run("bad TRYL offset 1", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[trylOff+1] = byte(-(trylOff - jmpOff) - 1) // into "something"
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("bad TRYL offset 2", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[trylOff+5] = byte(len(bad) - trylOff - 1)
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("bad ISTYPE type", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[istypeOff+1] = byte(0xff)
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("bad ISTYPE type (Any)", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[istypeOff+1] = byte(stackitem.AnyT)
		require.Error(t, IsScriptCorrect(bad, nil))
	})

	t.Run("good NEWARRAY_T type", func(t *testing.T) {
		bad := slices.Clone(good)
		bad[istypeOff] = byte(opcode.NEWARRAYT)
		bad[istypeOff+1] = byte(stackitem.AnyT)
		require.NoError(t, IsScriptCorrect(bad, nil))
	})

	t.Run("good methods", func(t *testing.T) {
		methods := bitfield.New(len(good))
		methods.Set(retOff)
		methods.Set(tryOff)
		methods.Set(pushOff)
		require.NoError(t, IsScriptCorrect(good, methods))
	})

	t.Run("bad methods", func(t *testing.T) {
		methods := bitfield.New(len(good))
		methods.Set(retOff)
		methods.Set(tryOff)
		methods.Set(pushOff + 1)
		require.Error(t, IsScriptCorrect(good, methods))
	})
}
