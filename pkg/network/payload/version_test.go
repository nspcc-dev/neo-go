package payload

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionEncodeDecode(t *testing.T) {
	var port uint16 = 3000
	var id uint32 = 13337
	useragent := "/NEO:0.0.1/"
	var height uint32 = 100500
	var relay bool = true

	version := NewVersion(id, port, useragent, height, relay)

	buf := new(bytes.Buffer)
	err := version.EncodeBinary(buf)
	assert.Nil(t, err)
	assert.Equal(t, int(version.Size()), buf.Len())

	versionDecoded := &Version{}
	err = versionDecoded.DecodeBinary(buf)
	assert.Nil(t, err)
	assert.Equal(t, versionDecoded.Nonce, id)
	assert.Equal(t, versionDecoded.Port, port)
	assert.Equal(t, versionDecoded.UserAgent, []byte(useragent))
	assert.Equal(t, versionDecoded.StartHeight, height)
	assert.Equal(t, versionDecoded.Relay, relay)
	assert.Equal(t, version, versionDecoded)
}
