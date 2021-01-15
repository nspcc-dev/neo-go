package vm

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type (
	iterator interface {
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

	mapWrapper struct {
		index int
		m     []stackitem.MapElement
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

func (m *mapWrapper) Next() bool {
	if next := m.index + 1; next < len(m.m) {
		m.index = next
		return true
	}

	return false
}

func (m *mapWrapper) Value() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		m.m[m.index].Key,
		m.m[m.index].Value,
	})
}
