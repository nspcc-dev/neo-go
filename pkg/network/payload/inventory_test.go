package payload

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	. "github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestInventoryEncodeDecode(t *testing.T) {
	hashes := []Uint256{
		hash.Sha256([]byte("a")),
		hash.Sha256([]byte("b")),
	}
	inv := NewInventory(BlockType, hashes)

	buf := new(bytes.Buffer)
	err := inv.EncodeBinary(buf)
	assert.Nil(t, err)

	invDecode := &Inventory{}
	err = invDecode.DecodeBinary(buf)
	assert.Nil(t, err)
	assert.Equal(t, inv, invDecode)
}

func TestEmptyInv(t *testing.T) {
	msgInv := NewInventory(TXType, []Uint256{})

	buf := new(bytes.Buffer)
	err := msgInv.EncodeBinary(buf)
	assert.Nil(t, err)
	assert.Equal(t, []byte{byte(TXType), 0}, buf.Bytes())
	assert.Equal(t, 0, len(msgInv.Hashes))
}
