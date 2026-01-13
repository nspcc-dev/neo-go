package state

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// WhitelistFeeContract represents a whitelisted contract method with fixed
// execution price. It implements [stackitem.Convertible] interface.
type WhitelistFeeContract struct {
	Hash   util.Uint160
	Method string
	ArgCnt int
	Fee    int64
}

var _ = stackitem.Convertible(&WhitelistFeeContract{})

// ToStackItem converts WhitelistFeeContract to stackitem.Item.
func (r *WhitelistFeeContract) ToStackItem() (stackitem.Item, error) {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray(r.Hash.BytesBE()),
		stackitem.NewByteArray([]byte(r.Method)),
		stackitem.NewBigInteger(big.NewInt(int64(r.ArgCnt))),
		stackitem.NewBigInteger(big.NewInt(r.Fee)),
	}), nil
}

// FromStackItem fills WhitelistFeeContract's data from the given stack
// itemized contract representation.
func (r *WhitelistFeeContract) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not a struct")
	}
	if len(arr) != 4 {
		return errors.New("invalid structure")
	}

	var err error
	r.Hash, err = stackitem.ToUint160(arr[0])
	if err != nil {
		return fmt.Errorf("invalid hash: %w", err)
	}
	r.Method, err = stackitem.ToString(arr[1])
	if err != nil {
		return fmt.Errorf("invalid method: %w", err)
	}
	argCnt, err := stackitem.ToInt32(arr[2])
	if err != nil {
		return fmt.Errorf("invalid argument count: %w", err)
	}
	r.ArgCnt = int(argCnt)
	r.Fee, err = stackitem.ToInt64(arr[3])
	if err != nil {
		return fmt.Errorf("invalid fee: %w", err)
	}

	return nil
}
