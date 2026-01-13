package state

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Deposit represents GAS deposit from a Notary contract.
type Deposit struct {
	Amount *big.Int
	Till   uint32
}

// ToStackItem implements stackitem.Convertible interface. It never returns an
// error.
func (d *Deposit) ToStackItem() (stackitem.Item, error) {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewBigInteger(d.Amount),
		stackitem.Make(d.Till),
	}), nil
}

// FromStackItem implements stackitem.Convertible interface.
func (d *Deposit) FromStackItem(it stackitem.Item) error {
	items, ok := it.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not a struct")
	}
	if len(items) != 2 {
		return errors.New("wrong number of elements")
	}
	amount, err := items[0].TryInteger()
	if err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}
	d.Amount = amount
	d.Till, err = stackitem.ToUint32(items[1])
	if err != nil {
		return fmt.Errorf("invalid till: %w", err)
	}
	return nil
}
