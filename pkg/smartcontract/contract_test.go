package smartcontract

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/stretchr/testify/assert"
)

func TestCreateMultiSigRedeemScript(t *testing.T) {
	val1, _ := keys.NewPublicKeyFromString("03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c")
	val2, _ := keys.NewPublicKeyFromString("02df48f60e8f3e01c48ff40b9b7f1310d7a8b2a193188befe1c2e3df740e895093")
	val3, _ := keys.NewPublicKeyFromString("03b8d9d5771d8f513aa0869b9cc8d50986403b78c6da36890638c3d46a5adce04a")

	validators := []*keys.PublicKey{val1, val2, val3}

	out, err := CreateMultiSigRedeemScript(3, validators)
	if err != nil {
		t.Fatal(err)
	}

	br := io.NewBinReaderFromBuf(out)
	var b uint8
	br.ReadLE(&b)
	assert.Equal(t, vm.PUSH3, vm.Instruction(b))

	for i := 0; i < len(validators); i++ {
		bb := br.ReadBytes()
		if br.Err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, validators[i].Bytes(), bb)
	}

	br.ReadLE(&b)
	assert.Equal(t, vm.PUSH3, vm.Instruction(b))
	br.ReadLE(&b)
	assert.Equal(t, vm.CHECKMULTISIG, vm.Instruction(b))
}
