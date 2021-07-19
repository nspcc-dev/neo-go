package native

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type candidate struct {
	Registered bool
	Votes      big.Int
}

// FromBytes unmarshals candidate from byte array.
func (c *candidate) FromBytes(data []byte) *candidate {
	err := stackitem.DeserializeConvertible(data, c)
	if err != nil {
		panic(err)
	}
	return c
}

// ToStackItem implements stackitem.Convertible. It never returns an error.
func (c *candidate) ToStackItem() (stackitem.Item, error) {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewBool(c.Registered),
		stackitem.NewBigInteger(&c.Votes),
	}), nil
}

// FromStackItem implements stackitem.Convertible.
func (c *candidate) FromStackItem(item stackitem.Item) error {
	arr := item.(*stackitem.Struct).Value().([]stackitem.Item)
	vs, err := arr[1].TryInteger()
	if err != nil {
		return err
	}
	reg, err := arr[0].TryBool()
	if err != nil {
		return err
	}
	c.Registered = reg
	c.Votes = *vs
	return nil
}
