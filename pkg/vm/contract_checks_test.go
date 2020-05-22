package vm

import (
	"encoding/binary"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSignatureContract() []byte {
	prog := make([]byte, 41)
	prog[0] = byte(opcode.PUSHDATA1)
	prog[1] = 33
	prog[35] = byte(opcode.PUSHNULL)
	prog[36] = byte(opcode.SYSCALL)
	binary.LittleEndian.PutUint32(prog[37:], verifyInteropID)
	return prog
}

func TestIsSignatureContract(t *testing.T) {
	t.Run("valid contract", func(t *testing.T) {
		prog := testSignatureContract()
		assert.True(t, IsSignatureContract(prog))
		assert.True(t, IsStandardContract(prog))
	})

	t.Run("invalid interop ID", func(t *testing.T) {
		prog := testSignatureContract()
		binary.LittleEndian.PutUint32(prog[37:], ^verifyInteropID)
		assert.False(t, IsSignatureContract(prog))
		assert.False(t, IsStandardContract(prog))
	})

	t.Run("invalid pubkey size", func(t *testing.T) {
		prog := testSignatureContract()
		prog[1] = 32
		assert.False(t, IsSignatureContract(prog))
		assert.False(t, IsStandardContract(prog))
	})

	t.Run("no PUSHNULL", func(t *testing.T) {
		prog := testSignatureContract()
		prog[35] = byte(opcode.PUSH1)
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

	t.Run("no PUSHNULL", func(t *testing.T) {
		prog := testMultisigContract(t, 2, 2)
		prog[len(prog)-6] ^= 0xFF
		assert.False(t, IsMultiSigContract(prog))
	})

	t.Run("invalid keys number", func(t *testing.T) {
		prog := testMultisigContract(t, 2, 2)
		prog[len(prog)-7] = byte(opcode.PUSH3)
		assert.False(t, IsMultiSigContract(prog))
	})

	t.Run("invalid length", func(t *testing.T) {
		prog := testMultisigContract(t, 2, 2)
		prog = append(prog, 0)
		assert.False(t, IsMultiSigContract(prog))
	})
}
