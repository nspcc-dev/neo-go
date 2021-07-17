package state

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Deposit represents GAS deposit from Notary contract.
type Deposit struct {
	Amount *big.Int
	Till   uint32
}

// EncodeBinary implements io.Serializable interface.
func (d *Deposit) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinary(d.toStackItem(), w)
}

// DecodeBinary implements io.Serializable interface.
func (d *Deposit) DecodeBinary(r *io.BinReader) {
	si := stackitem.DecodeBinary(r)
	if r.Err != nil {
		return
	}
	r.Err = d.fromStackItem(si)
}

func (d *Deposit) toStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewBigInteger(d.Amount),
		stackitem.Make(d.Till),
	})
}

func (d *Deposit) fromStackItem(it stackitem.Item) error {
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
	till, err := items[1].TryInteger()
	if err != nil {
		return fmt.Errorf("invalid till: %w", err)
	}
	ti64 := till.Int64()
	if !till.IsInt64() || ti64 > math.MaxUint32 || ti64 < 0 {
		return errors.New("wrong till value")
	}
	d.Amount = amount
	d.Till = uint32(ti64)
	return nil
}
