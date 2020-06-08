package vm

import (
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Slot is a fixed-size slice of stack items.
type Slot struct {
	storage []stackitem.Item
	refs    *refCounter
}

// newSlot returns new slot of n items.
func newSlot(n int, refs *refCounter) *Slot {
	return &Slot{
		storage: make([]stackitem.Item, n),
		refs:    refs,
	}
}

func (v *VM) newSlot(n int) *Slot {
	return newSlot(n, v.refs)
}

// Set sets i-th storage slot.
func (s *Slot) Set(i int, item stackitem.Item) {
	if s.storage[i] == item {
		return
	}
	old := s.storage[i]
	s.storage[i] = item
	if old != nil {
		s.refs.Remove(old)
	}
	s.refs.Add(item)
}

// Get returns item contained in i-th slot.
func (s *Slot) Get(i int) stackitem.Item {
	if item := s.storage[i]; item != nil {
		return item
	}
	return stackitem.Null{}
}

// Size returns slot size.
func (s *Slot) Size() int { return len(s.storage) }
