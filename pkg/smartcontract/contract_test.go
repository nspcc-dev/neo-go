package smartcontract

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateMultiSigRedeemScript(t *testing.T) {
	val1, _ := keys.NewPublicKeyFromString("03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c")
	val2, _ := keys.NewPublicKeyFromString("02df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e895093")
	val3, _ := keys.NewPublicKeyFromString("03b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a")

	validators := []*keys.PublicKey{val1, val2, val3}

	out, err := CreateMultiSigRedeemScript(3, validators)
	require.NoError(t, err)

	br := io.NewBinReaderFromBuf(out)
	assert.Equal(t, opcode.PUSH3, opcode.Opcode(br.ReadB()))

	for i := 0; i < len(validators); i++ {
		assert.EqualValues(t, opcode.PUSHDATA1, br.ReadB())
		bb := br.ReadVarBytes()
		require.NoError(t, br.Err)
		assert.Equal(t, validators[i].Bytes(), bb)
	}

	assert.Equal(t, opcode.PUSH3, opcode.Opcode(br.ReadB()))
	assert.Equal(t, opcode.SYSCALL, opcode.Opcode(br.ReadB()))
	assert.Equal(t, interopnames.ToID([]byte(interopnames.SystemCryptoCheckMultisig)), br.ReadU32LE())
}

func TestCreateDefaultMultiSigRedeemScript(t *testing.T) {
	var validators = make([]*keys.PublicKey, 0)

	var addKey = func() {
		key, err := keys.NewPrivateKey()
		require.NoError(t, err)
		validators = append(validators, key.PublicKey())
	}
	var checkM = func(m int) {
		validScript, err := CreateMultiSigRedeemScript(m, validators)
		require.NoError(t, err)
		defaultScript, err := CreateDefaultMultiSigRedeemScript(validators)
		require.NoError(t, err)
		require.Equal(t, validScript, defaultScript)
	}

	// 1 out of 1
	addKey()
	checkM(1)

	// 2 out of 2
	addKey()
	checkM(2)

	// 3 out of 4
	for i := 0; i < 2; i++ {
		addKey()
	}
	checkM(3)

	// 5 out of 6
	for i := 0; i < 2; i++ {
		addKey()
	}
	checkM(5)

	// 5 out of 7
	addKey()
	checkM(5)

	// 7 out of 10
	for i := 0; i < 3; i++ {
		addKey()
	}
	checkM(7)
}

func TestCreateMajorityMultiSigRedeemScript(t *testing.T) {
	var validators = make([]*keys.PublicKey, 0)

	var addKey = func() {
		key, err := keys.NewPrivateKey()
		require.NoError(t, err)
		validators = append(validators, key.PublicKey())
	}
	var checkM = func(m int) {
		validScript, err := CreateMultiSigRedeemScript(m, validators)
		require.NoError(t, err)
		defaultScript, err := CreateMajorityMultiSigRedeemScript(validators)
		require.NoError(t, err)
		require.Equal(t, validScript, defaultScript)
	}

	// 1 out of 1
	addKey()
	checkM(1)

	// 2 out of 2
	addKey()
	checkM(2)

	// 3 out of 4
	addKey()
	addKey()
	checkM(3)

	// 4 out of 6
	addKey()
	addKey()
	checkM(4)

	// 5 out of 8
	addKey()
	addKey()
	checkM(5)

	// 6 out of 10
	addKey()
	addKey()
	checkM(6)
}
