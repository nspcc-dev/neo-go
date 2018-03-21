package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodePublicKey(t *testing.T) {
	for i := 0; i < 4; i++ {
		p := &PublicKey{RandomECPoint()}
		buf := new(bytes.Buffer)
		assert.Nil(t, p.EncodeBinary(buf))

		pDecode := &PublicKey{}
		assert.Nil(t, pDecode.DecodeBinary(buf))
		assert.Equal(t, p.X, pDecode.X)
	}
}
