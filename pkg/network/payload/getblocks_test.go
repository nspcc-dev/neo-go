package payload

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
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
	buf := new(bytes.Buffer)
	if err := p.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	pDecode := &GetBlocks{}
	if err := pDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

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
	buf := new(bytes.Buffer)
	if err := p.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	pDecode := &GetBlocks{}
	if err := pDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, p, pDecode)
}
