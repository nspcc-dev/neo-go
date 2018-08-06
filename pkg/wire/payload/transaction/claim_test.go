package transaction

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeClaim(t *testing.T) {

	// test taken from mainnet: abf142faf539c340e42722b5b34b505cf4fd73185fed775784e37c2c5ef1b866
	rawtx := "020001af1b3a0f3729572893ce4e82f2113d18ec9a5e9d6fe02117eaa9e0c5a43770490000000001e72d286979ee6cb1b7e65dfddfb2e384100b8d148e7758de42e4168b71792c6048ae5801000000001123b6b74273562540479eea5cd0139f88ac7dd301414085f6d9edc24ab68c5d15c8e164de6702106c53bc15fa2c45b575bd3543c19132de61dd1922407be56affbcea73e5f8878811549340fd3c951e8593d51f3c8a962321028cf5e5a4d430db0202755c2cf1b3c99efcb4da4e41e182450dc5e1ddffb54bbfac"
	rawtxBytes, _ := hex.DecodeString(rawtx)

	c := NewClaim(0)

	r := bytes.NewReader(rawtxBytes)
	err := c.Decode(r)
	assert.Equal(t, nil, err)

	assert.Equal(t, types.Claim, c.Type)
	assert.Equal(t, 0, int(c.Version))
	assert.Equal(t, 1, int(len(c.Claims)))

	claim := c.Claims[0]
	assert.Equal(t, "497037a4c5e0a9ea1721e06f9d5e9aec183d11f2824ece93285729370f3a1baf", claim.PrevHash.String())
	assert.Equal(t, uint16(0), claim.PrevIndex)
	assert.Equal(t, "abf142faf539c340e42722b5b34b505cf4fd73185fed775784e37c2c5ef1b866", c.Hash.String())

	// Encode
	buf := new(bytes.Buffer)
	err = c.Encode(buf)
	assert.Equal(t, nil, err)
	assert.Equal(t, rawtx, hex.EncodeToString(buf.Bytes()))
}
