package smartcontract

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/stretchr/testify/assert"
)

func TestCreateMultiSigRedeemScript(t *testing.T) {
	val1, _ := crypto.NewPublicKeyFromString("03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c")
	val2, _ := crypto.NewPublicKeyFromString("02df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e895093")
	val3, _ := crypto.NewPublicKeyFromString("03b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a")

	validators := []*crypto.PublicKey{val1, val2, val3}

	out, err := CreateMultiSigRedeemScript(3, validators)
	if err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(out)
	b, _ := buf.ReadByte()
	assert.Equal(t, vm.Opush3, vm.Opcode(b))

	for i := 0; i < len(validators); i++ {
		b, err := util.ReadVarBytes(buf)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, validators[i].Bytes(), b)
	}

	b, _ = buf.ReadByte()
	assert.Equal(t, vm.Opush3, vm.Opcode(b))
	b, _ = buf.ReadByte()
	assert.Equal(t, vm.Ocheckmultisig, vm.Opcode(b))
}
