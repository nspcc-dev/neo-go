package iterator

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type iterator interface {
	Next() bool
	Value() stackitem.Item
}

// Next advances the iterator, pushes true on success and false otherwise.
func Next(ic *interop.Context) error {
	iop := ic.VM.Estack().Pop().Interop()
	arr := iop.Value().(iterator)
	ic.VM.Estack().PushItem(stackitem.Bool(arr.Next()))

	return nil
}

// Value returns current iterator value and depends on iterator type:
// For slices the result is just value.
// For maps the result is key-value pair packed in a struct.
func Value(ic *interop.Context) error {
	iop := ic.VM.Estack().Pop().Interop()
	arr := iop.Value().(iterator)
	ic.VM.Estack().PushItem(arr.Value())

	return nil
}

// IsIterator returns whether stackitem implements iterator interface.
func IsIterator(item stackitem.Item) bool {
	_, ok := item.Value().(iterator)
	return ok
}

// ValuesTruncated returns an array of up to `max` iterator values. The second
// return parameter denotes whether iterator is truncated, i.e. has more values.
// The provided iterator CAN NOT be reused in the subsequent calls to Values and
// to ValuesTruncated.
func ValuesTruncated(item stackitem.Item, max int) ([]stackitem.Item, bool) {
	result := Values(item, max)
	arr := item.Value().(iterator)
	return result, arr.Next()
}

// Values returns an array of up to `max` iterator values. The provided
// iterator can safely be reused to retrieve the rest of its values in the
// subsequent calls to Values and to ValuesTruncated.
func Values(item stackitem.Item, max int) []stackitem.Item {
	var result []stackitem.Item
	arr := item.Value().(iterator)
	for max > 0 && arr.Next() {
		result = append(result, arr.Value())
		max--
	}
	return result
}
