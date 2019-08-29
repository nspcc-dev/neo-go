package payload

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionEncodeDecode(t *testing.T) {
	version := NewVersion(13337, 3000, "/NEO:0.0.1/", 0, true)

	buf := new(bytes.Buffer)
	err := version.EncodeBinary(buf)
	assert.Nil(t, err)
	assert.Equal(t, int(version.Size()), buf.Len())

	versionDecoded := &Version{}
	err = versionDecoded.DecodeBinary(buf)
	assert.Nil(t, err)
	assert.Equal(t, version, versionDecoded)
}
