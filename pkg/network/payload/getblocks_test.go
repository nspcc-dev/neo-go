package payload

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestGetBlockEncodeDecode(t *testing.T) {
	start := []util.Uint256{
		hash.Sha256([]byte("a")),
		hash.Sha256([]byte("b")),
		hash.Sha256([]byte("c")),
		hash.Sha256([]byte("d")),
	}

	p := NewGetBlocks(start, util.Uint256{})
	buf := io.NewBufBinWriter()
	err := p.EncodeBinary(buf.BinWriter)
	assert.Nil(t, err)

	b := buf.Bytes()
	r := io.NewBinReaderFromBuf(b)
	pDecode := &GetBlocks{}
	err = pDecode.DecodeBinary(r)
	assert.Nil(t, err)
	assert.Equal(t, p, pDecode)
}

func TestGetBlockEncodeDecodeWithHashStop(t *testing.T) {
	var (
		start = []util.Uint256{
			hash.Sha256([]byte("a")),
			hash.Sha256([]byte("b")),
			hash.Sha256([]byte("c")),
			hash.Sha256([]byte("d")),
		}
		stop = hash.Sha256([]byte("e"))
	)
	p := NewGetBlocks(start, stop)
	buf := io.NewBufBinWriter()
	err := p.EncodeBinary(buf.BinWriter)
	assert.Nil(t, err)

	b := buf.Bytes()
	r := io.NewBinReaderFromBuf(b)
	pDecode := &GetBlocks{}
	err = pDecode.DecodeBinary(r)
	assert.Nil(t, err)
	assert.Equal(t, p, pDecode)
}
