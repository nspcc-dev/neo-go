package core

import (
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// StorageItem is the value to be stored with read-only flag.
type StorageItem struct {
	Value   []byte
	IsConst bool
}

// makeStorageItemKey returns a key used to store StorageItem in the DB.
func makeStorageItemKey(scripthash util.Uint160, key []byte) []byte {
	return storage.AppendPrefix(storage.STStorage, append(scripthash.BytesReverse(), key...))
}

// getStorageItemFromStore returns StorageItem if it exists in the given Store.
func getStorageItemFromStore(s storage.Store, scripthash util.Uint160, key []byte) *StorageItem {
	b, err := s.Get(makeStorageItemKey(scripthash, key))
	if err != nil {
		return nil
	}
	r := io.NewBinReaderFromBuf(b)

	si := &StorageItem{}
	si.DecodeBinary(r)
	if r.Err != nil {
		return nil
	}

	return si
}

// putStorageItemIntoStore puts given StorageItem for given script with given
// key into the given Store.
func putStorageItemIntoStore(s storage.Store, scripthash util.Uint160, key []byte, si *StorageItem) error {
	buf := io.NewBufBinWriter()
	si.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return s.Put(makeStorageItemKey(scripthash, key), buf.Bytes())
}

// deleteStorageItemInStore drops storage item for the given script with the
// given key from the Store.
func deleteStorageItemInStore(s storage.Store, scripthash util.Uint160, key []byte) error {
	return s.Delete(makeStorageItemKey(scripthash, key))
}

// EncodeBinary implements Serializable interface.
func (si *StorageItem) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(si.Value)
	w.WriteLE(si.IsConst)
}

// DecodeBinary implements Serializable interface.
func (si *StorageItem) DecodeBinary(r *io.BinReader) {
	si.Value = r.ReadBytes()
	r.ReadLE(&si.IsConst)
}
