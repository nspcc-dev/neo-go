package payload

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	. "github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestInventoryEncodeDecode(t *testing.T) {
	hashes := []Uint256{
		hash.Sha256([]byte("a")),
		hash.Sha256([]byte("b")),
	}
	inv := NewInventory(BlockType, hashes)

	buf := io.NewBufBinWriter()
	err := inv.EncodeBinary(buf.BinWriter)
	assert.Nil(t, err)

	b := buf.Bytes()
	r := io.NewBinReaderFromBuf(b)
	invDecode := &Inventory{}
	err = invDecode.DecodeBinary(r)
	assert.Nil(t, err)
	assert.Equal(t, inv, invDecode)
}

func TestEmptyInv(t *testing.T) {
	msgInv := NewInventory(TXType, []Uint256{})

	buf := io.NewBufBinWriter()
	err := msgInv.EncodeBinary(buf.BinWriter)
	assert.Nil(t, err)
	assert.Equal(t, []byte{byte(TXType), 0}, buf.Bytes())
	assert.Equal(t, 0, len(msgInv.Hashes))
}
