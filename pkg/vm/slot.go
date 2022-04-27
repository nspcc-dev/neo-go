package vm

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// slot is a fixed-size slice of stack items.
type slot []stackitem.Item

// init sets static slot size to n. It is intended to be used only by INITSSLOT.
func (s *slot) init(n int) {
	if *s != nil {
		panic("already initialized")
	}
	*s = make([]stackitem.Item, n)
}

// Set sets i-th storage slot.
func (s slot) Set(i int, item stackitem.Item, refs *refCounter) {
	if s[i] == item {
		return
	}
	old := s[i]
	s[i] = item
	if old != nil {
		refs.Remove(old)
	}
	refs.Add(item)
}

// Get returns the item contained in the i-th slot.
func (s slot) Get(i int) stackitem.Item {
	if item := s[i]; item != nil {
		return item
	}
	return stackitem.Null{}
}

// Clear removes all slot variables from the reference counter.
func (s slot) Clear(refs *refCounter) {
	for _, item := range s {
		refs.Remove(item)
	}
}

// Size returns the slot size.
func (s slot) Size() int {
	if s == nil {
		panic("not initialized")
	}
	return len(s)
}

// MarshalJSON implements the JSON marshalling interface.
func (s slot) MarshalJSON() ([]byte, error) {
	arr := make([]json.RawMessage, len(s))
	for i := range s {
		data, err := stackitem.ToJSONWithTypes(s[i])
		if err == nil {
			arr[i] = data
		}
	}
	return json.Marshal(arr)
}
