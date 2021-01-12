package vm

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type (
	enumerator interface {
		Next() bool
		Value() stackitem.Item
	}

	arrayWrapper struct {
		index int
		value []stackitem.Item
	}

	byteArrayWrapper struct {
		index int
		value []byte
	}

)

type (
	iterator interface {
		enumerator
		Key() stackitem.Item
	}

	mapWrapper struct {
		index int
		m     []stackitem.MapElement
	}

	keysWrapper struct {
		iter iterator
	}

	valuesWrapper struct {
		iter iterator
	}
)

func (a *arrayWrapper) Next() bool {
	if next := a.index + 1; next < len(a.value) {
		a.index = next
		return true
	}

	return false
}

func (a *arrayWrapper) Value() stackitem.Item {
	return a.value[a.index]
}

func (a *arrayWrapper) Key() stackitem.Item {
	return stackitem.Make(a.index)
}

func (a *byteArrayWrapper) Next() bool {
	if next := a.index + 1; next < len(a.value) {
		a.index = next
		return true
	}

	return false
}

func (a *byteArrayWrapper) Value() stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(a.value[a.index])))
}

func (a *byteArrayWrapper) Key() stackitem.Item {
	return stackitem.Make(a.index)
}

func (m *mapWrapper) Next() bool {
	if next := m.index + 1; next < len(m.m) {
		m.index = next
		return true
	}

	return false
}

func (m *mapWrapper) Value() stackitem.Item {
	return m.m[m.index].Value
}

func (m *mapWrapper) Key() stackitem.Item {
	return m.m[m.index].Key
}

func (e *keysWrapper) Next() bool {
	return e.iter.Next()
}

func (e *keysWrapper) Value() stackitem.Item {
	return e.iter.Key()
}

func (e *valuesWrapper) Next() bool {
	return e.iter.Next()
}

func (e *valuesWrapper) Value() stackitem.Item {
	return e.iter.Value()
}
