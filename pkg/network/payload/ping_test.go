package payload

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeBinary(t *testing.T) {
	payload := NewPing(uint32(1), uint32(2))
	assert.NotEqual(t, 0, payload.Timestamp)

	bufBinWriter := io.NewBufBinWriter()
	payload.EncodeBinary(bufBinWriter.BinWriter)
	assert.Nil(t, bufBinWriter.Err)

	binReader := io.NewBinReaderFromBuf(bufBinWriter.Bytes())
	decodedPing := &Ping{}
	decodedPing.DecodeBinary(binReader)
	assert.Nil(t, binReader.Err)

	assert.Equal(t, uint32(1), decodedPing.LastBlockIndex)
	assert.Equal(t, uint32(2), decodedPing.Nonce)
}
