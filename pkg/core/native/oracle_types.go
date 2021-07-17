package native

import (
	"crypto/elliptic"
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// IDList is a list of oracle request IDs.
type IDList []uint64

// NodeList represents list or oracle nodes.
type NodeList keys.PublicKeys

// ToStackItem implements stackitem.Convertible. It never returns an error.
func (l IDList) ToStackItem() (stackitem.Item, error) {
	arr := make([]stackitem.Item, len(l))
	for i := range l {
		arr[i] = stackitem.NewBigInteger(new(big.Int).SetUint64(l[i]))
	}
	return stackitem.NewArray(arr), nil
}

// FromStackItem implements stackitem.Convertible.
func (l *IDList) FromStackItem(it stackitem.Item) error {
	arr, ok := it.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	*l = make(IDList, len(arr))
	for i := range arr {
		bi, err := arr[i].TryInteger()
		if err != nil {
			return err
		}
		(*l)[i] = bi.Uint64()
	}
	return nil
}

// Remove removes id from list.
func (l *IDList) Remove(id uint64) bool {
	for i := range *l {
		if id == (*l)[i] {
			if i < len(*l) {
				copy((*l)[i:], (*l)[i+1:])
			}
			*l = (*l)[:len(*l)-1]
			return true
		}
	}
	return false
}

// ToStackItem implements stackitem.Convertible. It never returns an error.
func (l NodeList) ToStackItem() (stackitem.Item, error) {
	arr := make([]stackitem.Item, len(l))
	for i := range l {
		arr[i] = stackitem.NewByteArray(l[i].Bytes())
	}
	return stackitem.NewArray(arr), nil
}

// FromStackItem implements stackitem.Convertible.
func (l *NodeList) FromStackItem(it stackitem.Item) error {
	arr, ok := it.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	*l = make(NodeList, len(arr))
	for i := range arr {
		bs, err := arr[i].TryBytes()
		if err != nil {
			return err
		}
		pub, err := keys.NewPublicKeyFromBytes(bs, elliptic.P256())
		if err != nil {
			return err
		}
		(*l)[i] = pub
	}
	return nil
}
