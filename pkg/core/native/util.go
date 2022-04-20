package native

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var intOne = big.NewInt(1)
var intTwo = big.NewInt(2)

func getConvertibleFromDAO(id int32, d *dao.Simple, key []byte, conv stackitem.Convertible) error {
	si := d.GetStorageItem(id, key)
	if si == nil {
		return storage.ErrKeyNotFound
	}
	return stackitem.DeserializeConvertible(si, conv)
}

func putConvertibleToDAO(id int32, d *dao.Simple, key []byte, conv stackitem.Convertible) error {
	data, err := stackitem.SerializeConvertible(conv)
	if err != nil {
		return err
	}
	d.PutStorageItem(id, key, data)
	return nil
}

func setIntWithKey(id int32, dao *dao.Simple, key []byte, value int64) {
	dao.PutStorageItem(id, key, bigint.ToBytes(big.NewInt(value)))
}

func getIntWithKey(id int32, dao *dao.Simple, key []byte) int64 {
	si := dao.GetStorageItem(id, key)
	if si == nil {
		panic(fmt.Errorf("item with id = %d and key = %s is not initialized", id, hex.EncodeToString(key)))
	}
	return bigint.FromBytes(si).Int64()
}

// makeUint160Key creates a key from the account script hash.
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
