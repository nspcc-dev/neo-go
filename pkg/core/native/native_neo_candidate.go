package native

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type candidate struct {
	Registered bool
	Votes      big.Int
}

// Bytes marshals c to byte array.
func (c *candidate) Bytes() []byte {
	res, err := stackitem.Serialize(c.toStackItem())
	if err != nil {
		panic(err)
	}
	return res
}

// FromBytes unmarshals candidate from byte array.
func (c *candidate) FromBytes(data []byte) *candidate {
	item, err := stackitem.Deserialize(data)
	if err != nil {
		panic(err)
	}
	return c.fromStackItem(item)
}

func (c *candidate) toStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewBool(c.Registered),
		stackitem.NewBigInteger(&c.Votes),
	})
}

func (c *candidate) fromStackItem(item stackitem.Item) *candidate {
	arr := item.(*stackitem.Struct).Value().([]stackitem.Item)
	vs, err := arr[1].TryInteger()
	if err != nil {
		panic(err)
	}
	c.Registered, err = arr[0].TryBool()
	if err != nil {
		panic(err)
	}
	c.Votes = *vs
	return c
}
