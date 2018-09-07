package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeRegister(t *testing.T) {
	// transaction taken from neo-python; can be found on testnet 0c092117b4ba47b81001712425e6e7f760a637695eaf23741ba335925b195ecd

	rawtx := "400060245b7b226c616e67223a227a682d434e222c226e616d65223a2254657374436f696e227d5dffffffffffffffff08034b44ed9c8a88fb2497b6b57206cc08edd42c5614bd1fee790e5b795dee0f4e1167f97110a66136d38badc7b9f88eab013027ce4900014423a26aeca49cdeeb9522c720e1ae3a93bbe27d53662839b16a438305c20906010001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c60001e1a210b00000067f97110a66136d38badc7b9f88eab013027ce490141405d8223ec807e3416a220a75ef9805dfa2e36bd4f6dcc7372373aa45f15c7fadfc96a8642e52acf56c2c66d549be4ba820484873d5cada00b9c1ce9674fbf96382321034b44ed9c8a88fb2497b6b57206cc08edd42c5614bd1fee790e5b795dee0f4e11ac"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	reg := NewRegister(0)

	r := bytes.NewReader(rawtxBytes)
	err := reg.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Register, reg.Type)

	buf := new(bytes.Buffer)
	err = reg.Encode(buf)

	assert.Equal(t, nil, err)
	assert.Equal(t, rawtx, hex.EncodeToString(buf.Bytes()))
	assert.Equal(t, "0c092117b4ba47b81001712425e6e7f760a637695eaf23741ba335925b195ecd", reg.Hash.String())
}
func TestEncodeDecodeGenesisRegister(t *testing.T) {

	// genesis transaction taken from mainnet; can be found on mainnet(Block 0) : c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b

	rawtx := "400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	reg := NewRegister(0)

	r := bytes.NewReader(rawtxBytes)
	err := reg.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Register, reg.Type)

	buf := new(bytes.Buffer)
	err = reg.Encode(buf)

	assert.Equal(t, nil, err)
	assert.Equal(t, rawtx, hex.EncodeToString(buf.Bytes()))
	assert.Equal(t, "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b", reg.Hash.String())
}
