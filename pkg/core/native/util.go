package native

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var intOne = big.NewInt(1)
var intTwo = big.NewInt(2)

func setIntWithKey(id int32, dao *dao.Simple, key []byte, value int64) {
	dao.PutBigInt(id, key, big.NewInt(value))
}

func getIntWithKey(id int32, dao *dao.Simple, key []byte) int64 {
	res, err := dao.GetInt(id, key)
	if err != nil {
		panic(err)
	}
	return res
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

func toBigInt(s stackitem.Item) *big.Int {
	bi, err := s.TryInteger()
	if err != nil {
		panic(err)
	}
	return bi
}

func toUint160(s stackitem.Item) util.Uint160 {
	u, err := stackitem.ToUint160(s)
	if err != nil {
		panic(err)
	}
	return u
}

func toUint32(s stackitem.Item) uint32 {
	i, err := stackitem.ToUint32(s)
	if err != nil {
		panic(err)
	}
	return i
}

func toUint8(s stackitem.Item) uint8 {
	i, err := stackitem.ToUint8(s)
	if err != nil {
		panic(err)
	}
	return i
}

func toInt64(s stackitem.Item) int64 {
	i, err := stackitem.ToInt64(s)
	if err != nil {
		panic(err)
	}
	return i
}

func toLimitedBytes(item stackitem.Item) []byte {
	src := toBytes(item)
	if len(src) > stdMaxInputLength {
		panic(ErrTooBigInput)
	}
	return src
}

func toBytes(item stackitem.Item) []byte {
	src, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	return src
}

func toLimitedString(item stackitem.Item) string {
	src := toString(item)
	if len(src) > stdMaxInputLength {
		panic(ErrTooBigInput)
	}
	return src
}
