package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeState(t *testing.T) {

	// transaction taken from testnet 8abf5ebdb9a8223b12109513647f45bd3c0a6cf1a6346d56684cff71ba308724
	rawtx := "900001482103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c10a5265676973746572656401010001cb4184f0a96e72656c1fbdd4f75cca567519e909fd43cefcec13d6c6abcb92a1000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6000b8fb050109000071f9cf7f0ec74ec0b0f28a92b12e1081574c0af00141408780d7b3c0aadc5398153df5e2f1cf159db21b8b0f34d3994d865433f79fafac41683783c48aef510b67660e3157b701b9ca4dd9946a385d578fba7dd26f4849232103c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	s := NewStateTX(0)

	r := bytes.NewReader(rawtxBytes)
	err := s.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.State, s.Type)

	assert.Equal(t, 1, len(s.Inputs))
	input := s.Inputs[0]
	assert.Equal(t, "a192cbabc6d613ecfcce43fd09e9197556ca5cf7d4bd1f6c65726ea9f08441cb", input.PrevHash.String())
	assert.Equal(t, uint16(0), input.PrevIndex)

	assert.Equal(t, 1, len(s.Descriptors))
	descriptor := s.Descriptors[0]
	assert.Equal(t, "03c089d7122b840a4935234e82e26ae5efd0c2acb627239dc9f207311337b6f2c1", hex.EncodeToString(descriptor.Key))
	assert.Equal(t, "52656769737465726564", hex.EncodeToString(descriptor.Value))
	assert.Equal(t, "\x01", descriptor.Field)
	assert.Equal(t, Validator, descriptor.Type)

	// Encode

	buf := new(bytes.Buffer)

	err = s.Encode(buf)

	assert.Equal(t, nil, err)
	assert.Equal(t, rawtxBytes, buf.Bytes())
	assert.Equal(t, "8abf5ebdb9a8223b12109513647f45bd3c0a6cf1a6346d56684cff71ba308724", s.Hash.String())
}
