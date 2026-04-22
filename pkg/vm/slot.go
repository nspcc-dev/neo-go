package vm

import (
	"encoding/json"
	"fmt"

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

// initFromStack is an optimized version of init() for cases where some
// elements are taken from the stack. This operation doesn't change the
// reference counter since items that were referenced by the stack will be
// referenced by the slot.
func (s *Slot) initFromStack(n int, t *Stack) {
	if *s != nil {
		panic("already initialized")
	}
	*s = make([]stackitem.Item, n)
	for i := range n {
		(*s)[i] = t.popNoRef().Item()
	}
}

// store stores the top element from the stack into the i-th slot. It modifies
// the reference counter accordingly.
func (s Slot) store(stack *Stack, i int, refs *refCounter) {
	if s == nil {
		panic("slot is not initialized")
	}
	if i < 0 || i >= len(s) {
		panic(fmt.Errorf("slot index is out of range: %d with length %d", i, len(s)))
	}
	item := stack.popNoRef().Item()
	refs.Remove(s[i])
	s[i] = item
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
