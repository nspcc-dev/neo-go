package vm

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Slot is a fixed-size slice of stack items.
type Slot []stackitem.Item

// init sets static slot size to n. It is intended to be used only by INITSSLOT.
func (s *Slot) init(n int, rc *refCounter) {
	if *s != nil {
		panic("already initialized")
	}
	*s = make([]stackitem.Item, n)
	*rc += refCounter(n) // Virtual "Null" elements.
}

// set sets i-th storage slot.
func (s Slot) set(i int, item stackitem.Item, refs *refCounter) {
	if s[i] == item {
		return
	}
	refs.Remove(s[i])
	s[i] = item
	refs.Add(item)
}

// Get returns the item contained in the i-th slot.
func (s Slot) Get(i int) stackitem.Item {
	if item := s[i]; item != nil {
		return item
	}
	return stackitem.Null{}
}

// clearRefs removes all slot variables from the reference counter.
func (s Slot) clearRefs(refs *refCounter) {
	for _, item := range s {
		refs.Remove(item)
	}
}

// Size returns the slot size.
func (s Slot) Size() int {
	if s == nil {
		return 0
	}
	return len(s)
}

// MarshalJSON implements the JSON marshalling interface.
func (s Slot) MarshalJSON() ([]byte, error) {
	arr := make([]json.RawMessage, len(s))
	for i := range s {
		data, err := stackitem.ToJSONWithTypes(s[i])
		if err == nil {
			arr[i] = data
		}
	}
	return json.Marshal(arr)
}
