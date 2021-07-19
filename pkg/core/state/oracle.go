package state

import (
	"errors"
	"math/big"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// OracleRequest represents oracle request.
type OracleRequest struct {
	OriginalTxID     util.Uint256
	GasForResponse   uint64
	URL              string
	Filter           *string
	CallbackContract util.Uint160
	CallbackMethod   string
	UserData         []byte
}

// ToStackItem implements stackitem.Convertible interface. It never returns an
// error.
func (o *OracleRequest) ToStackItem() (stackitem.Item, error) {
	filter := stackitem.Item(stackitem.Null{})
	if o.Filter != nil {
		filter = stackitem.Make(*o.Filter)
	}
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(o.OriginalTxID.BytesBE()),
		stackitem.NewBigInteger(new(big.Int).SetUint64(o.GasForResponse)),
		stackitem.Make(o.URL),
		filter,
		stackitem.NewByteArray(o.CallbackContract.BytesBE()),
		stackitem.Make(o.CallbackMethod),
		stackitem.NewByteArray(o.UserData),
	}), nil
}

// FromStackItem implements stackitem.Convertible interface.
func (o *OracleRequest) FromStackItem(it stackitem.Item) error {
	arr, ok := it.Value().([]stackitem.Item)
	if !ok || len(arr) < 7 {
		return errors.New("not an array of needed length")
	}
	bs, err := arr[0].TryBytes()
	if err != nil {
		return err
	}
	o.OriginalTxID, err = util.Uint256DecodeBytesBE(bs)
	if err != nil {
		return err
	}

	gas, err := arr[1].TryInteger()
	if err != nil {
		return err
	}
	o.GasForResponse = gas.Uint64()

	s, isNull, ok := itemToString(arr[2])
	if !ok || isNull {
		return errors.New("invalid URL")
	}
	o.URL = s

	s, isNull, ok = itemToString(arr[3])
	if !ok {
		return errors.New("invalid filter")
	} else if !isNull {
		filter := s
		o.Filter = &filter
	}

	bs, err = arr[4].TryBytes()
	if err != nil {
		return err
	}
	o.CallbackContract, err = util.Uint160DecodeBytesBE(bs)
	if err != nil {
		return err
	}

	o.CallbackMethod, isNull, ok = itemToString(arr[5])
	if !ok || isNull {
		return errors.New("invalid callback method")
	}

	o.UserData, err = arr[6].TryBytes()
	return err
}

func itemToString(it stackitem.Item) (string, bool, bool) {
	_, ok := it.(stackitem.Null)
	if ok {
		return "", true, true
	}
	bs, err := it.TryBytes()
	if err != nil || !utf8.Valid(bs) {
		return "", false, false
	}
	return string(bs), false, true
}
