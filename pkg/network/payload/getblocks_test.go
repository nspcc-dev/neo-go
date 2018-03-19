package payload

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestGetBlockEncodeDecode(t *testing.T) {
	start := []util.Uint256{
		sha256.Sum256([]byte("a")),
		sha256.Sum256([]byte("b")),
		sha256.Sum256([]byte("c")),
		sha256.Sum256([]byte("d")),
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
			sha256.Sum256([]byte("a")),
			sha256.Sum256([]byte("b")),
			sha256.Sum256([]byte("c")),
			sha256.Sum256([]byte("d")),
		}
		stop = sha256.Sum256([]byte("e"))
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
