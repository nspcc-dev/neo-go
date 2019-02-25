package payload

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/stretchr/testify/assert"
)

// Test taken from neo-go v1
func TestGetHeadersEncodeDecode(t *testing.T) {

	var (
		start = []util.Uint256{
			sha256.Sum256([]byte("a")),
			sha256.Sum256([]byte("b")),
			sha256.Sum256([]byte("c")),
			sha256.Sum256([]byte("d")),
		}
		stop = sha256.Sum256([]byte("e"))
	)
	msgGetHeaders, err := NewGetHeadersMessage(start, stop)
	assert.Equal(t, nil, err)

	buf := new(bytes.Buffer)

	err = msgGetHeaders.EncodePayload(buf)
	assert.Equal(t, nil, err)
	expected := checksum.FromBuf(buf)

	msgGetHeadersDec, err := NewGetHeadersMessage([]util.Uint256{}, util.Uint256{})
	assert.Equal(t, nil, err)

	r := bytes.NewReader(buf.Bytes())
	err = msgGetHeadersDec.DecodePayload(r)
	assert.Equal(t, nil, err)

	buf = new(bytes.Buffer)
	err = msgGetHeadersDec.EncodePayload(buf)
	have := checksum.FromBuf(buf)

	assert.Equal(t, expected, have)
}
