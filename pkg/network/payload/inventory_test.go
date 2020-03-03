package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	. "github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestInventoryEncodeDecode(t *testing.T) {
	hashes := []Uint256{
		hash.Sha256([]byte("a")),
		hash.Sha256([]byte("b")),
	}
	inv := NewInventory(BlockType, hashes)

	buf := io.NewBufBinWriter()
	inv.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)

	b := buf.Bytes()
	r := io.NewBinReaderFromBuf(b)
	invDecode := &Inventory{}
	invDecode.DecodeBinary(r)
	assert.Nil(t, r.Err)
	assert.Equal(t, inv, invDecode)
}

func TestEmptyInv(t *testing.T) {
	msgInv := NewInventory(TXType, []Uint256{})

	buf := io.NewBufBinWriter()
	msgInv.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	assert.Equal(t, []byte{byte(TXType), 0}, buf.Bytes())
	assert.Equal(t, 0, len(msgInv.Hashes))
}
