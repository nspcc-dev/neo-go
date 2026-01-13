package state

import (
	"errors"
	"fmt"
	"math/big"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// OracleRequest represents an oracle request.
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
	var err error
	o.OriginalTxID, err = stackitem.ToUint256(arr[0])
	if err != nil {
		return fmt.Errorf("invalid tx ID: %w", err)
	}

	o.GasForResponse, err = stackitem.ToUint64(arr[1])
	if err != nil {
		return fmt.Errorf("invalid gas for response: %w", err)
	}

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

	o.CallbackContract, err = stackitem.ToUint160(arr[4])
	if err != nil {
		return fmt.Errorf("invalid callback contract: %w", err)
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
