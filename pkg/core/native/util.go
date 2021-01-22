package native

import (
	"encoding/binary"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

func getInt64WithKey(id int32, d dao.DAO, key []byte, defaultValue int64) int64 {
	si := d.GetStorageItem(id, key)
	if si == nil {
		return defaultValue
	}
	return int64(binary.LittleEndian.Uint64(si.Value))
}

func setInt64WithKey(id int32, dao dao.DAO, key []byte, value int64) error {
	si := &state.StorageItem{
		Value: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(si.Value, uint64(value))
	return dao.PutStorageItem(id, key, si)
}

func getUint32WithKey(id int32, dao dao.DAO, key []byte, defaultValue uint32) uint32 {
	si := dao.GetStorageItem(id, key)
	if si == nil {
		return defaultValue
	}
	return binary.LittleEndian.Uint32(si.Value)
}

func setUint32WithKey(id int32, dao dao.DAO, key []byte, value uint32) error {
	si := &state.StorageItem{
		Value: make([]byte, 4),
	}
	binary.LittleEndian.PutUint32(si.Value, value)
	return dao.PutStorageItem(id, key, si)
}

// makeUint160Key creates a key from account script hash.
func makeUint160Key(prefix byte, h util.Uint160) []byte {
	k := make([]byte, util.Uint160Size+1)
	k[0] = prefix
	copy(k[1:], h.BytesBE())
	return k
}

func toString(item stackitem.Item) string {
	s, err := stackitem.ToString(item)
	if err != nil {
		panic(err)
	}
	return s
}
