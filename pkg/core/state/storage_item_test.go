package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
)

func TestEncodeDecodeStorageItem(t *testing.T) {
	storageItem := &StorageItem{
		Value: []byte{1, 2, 3},
	}

	testserdes.EncodeDecodeBinary(t, storageItem, new(StorageItem))
}
