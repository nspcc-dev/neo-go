package vm

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Slot is a fixed-size slice of stack items.
type Slot struct {
	storage []stackitem.Item
	refs    *refCounter
}

// newSlot returns new slot with the provided reference counter.
func newSlot(refs *refCounter) *Slot {
	return &Slot{
		refs: refs,
	}
}

// init sets static slot size to n. It is intended to be used only by INITSSLOT.
func (s *Slot) init(n int) {
	if s.storage != nil {
		panic("already initialized")
	}
	s.storage = make([]stackitem.Item, n)
}

func (v *VM) newSlot(n int) *Slot {
	s := newSlot(&v.refs)
	s.init(n)
	return s
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

// Clear removes all slot variables from reference counter.
func (s *Slot) Clear() {
	for _, item := range s.storage {
		s.refs.Remove(item)
	}
}

// Size returns slot size.
func (s *Slot) Size() int {
	if s.storage == nil {
		panic("not initialized")
	}
	return len(s.storage)
}

// MarshalJSON implements JSON marshalling interface.
func (s *Slot) MarshalJSON() ([]byte, error) {
	items := s.storage
	arr := make([]json.RawMessage, len(items))
	for i := range items {
		data, err := stackitem.ToJSONWithTypes(items[i])
		if err == nil {
			arr[i] = data
		}
	}
	return json.Marshal(arr)
}
