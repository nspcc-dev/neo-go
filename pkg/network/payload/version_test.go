package payload

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestVersionEncodeDecode(t *testing.T) {
	var port uint16 = 3000
	var id uint32 = 13337
	useragent := "/NEO:0.0.1/"
	var height uint32 = 100500
	var relay = true

	version := NewVersion(id, port, useragent, height, relay)

	buf := io.NewBufBinWriter()
	version.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	b := buf.Bytes()
	assert.Equal(t, io.GetVarSize(version), len(b))

	r := io.NewBinReaderFromBuf(b)
	versionDecoded := &Version{}
	versionDecoded.DecodeBinary(r)
	assert.Nil(t, r.Err)
	assert.Equal(t, versionDecoded.Nonce, id)
	assert.Equal(t, versionDecoded.Port, port)
	assert.Equal(t, versionDecoded.UserAgent, []byte(useragent))
	assert.Equal(t, versionDecoded.StartHeight, height)
	assert.Equal(t, versionDecoded.Relay, relay)
	assert.Equal(t, version, versionDecoded)
}
