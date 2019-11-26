package entities

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeStorageItem(t *testing.T) {
	storageItem := &StorageItem{
		Value:   []byte{},
		IsConst: false,
	}
	buf := io.NewBufBinWriter()
	storageItem.EncodeBinary(buf.BinWriter)
	require.NoError(t, buf.Err)

	decodedStorageItem := &StorageItem{}
	r := io.NewBinReaderFromBuf(buf.Bytes())
	decodedStorageItem.DecodeBinary(r)
	require.NoError(t, r.Err)

	assert.Equal(t, storageItem, decodedStorageItem)
}
