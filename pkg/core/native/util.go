package native

import (
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

func getSerializableFromDAO(id int32, d dao.DAO, key []byte, item io.Serializable) error {
	si := d.GetStorageItem(id, key)
	if si == nil {
		return storage.ErrKeyNotFound
	}
	r := io.NewBinReaderFromBuf(si.Value)
	item.DecodeBinary(r)
	return r.Err
}

func putSerializableToDAO(id int32, d dao.DAO, key []byte, item io.Serializable) error {
	w := io.NewBufBinWriter()
	item.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	return d.PutStorageItem(id, key, &state.StorageItem{
		Value: w.Bytes(),
	})
}
