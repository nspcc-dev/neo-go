package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeBinary(t *testing.T) {
	payload := NewPing(uint32(1), uint32(2))
	assert.NotEqual(t, 0, payload.Timestamp)

	decodedPing := &Ping{}
	testserdes.EncodeDecodeBinary(t, payload, decodedPing)

	assert.Equal(t, uint32(1), decodedPing.LastBlockIndex)
	assert.Equal(t, uint32(2), decodedPing.Nonce)
}
