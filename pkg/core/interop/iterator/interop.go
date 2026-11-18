package iterator

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type iterator interface {
	Next() bool
	// Value returns the current item and its deserialized ByteArray/Buffer byte
	// count, or zero if no deserialization is needed.
	Value() (stackitem.Item, int)
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
	v, _ := arr.Value()
	ic.VM.Estack().PushItem(v)

	return nil
}

// IsIterator returns whether stackitem implements iterator interface.
func IsIterator(item stackitem.Item) bool {
	_, ok := item.Value().(iterator)
	return ok
}

// ValuesTruncated returns an array of up to `maxNum` iterator values. The second
// return parameter denotes whether iterator is truncated, i.e. has more values.
// The third return parameter is the next value in the iterator. If the iterator
// doesn't have more values, then third return parameter is nil. The iterator can
// be reused for subsequent traversal, but `curr` will not be automatically
// included in subsequent calls to Values or ValuesTruncated.
func ValuesTruncated(item stackitem.Item, maxNum int) ([]stackitem.Item, bool, stackitem.Item) {
	arr, ok := item.Value().(iterator)
	if !ok {
		return nil, false, nil
	}
	result := Values(item, maxNum)
	if arr.Next() {
		curr, _ := arr.Value()
		return result, true, curr
	}
	return result, false, nil
}

// Values returns an array of up to `maxNum` iterator values. The provided
// iterator can safely be reused to retrieve the rest of its values in the
// subsequent calls to Values and to ValuesTruncated.
func Values(item stackitem.Item, maxNum int) []stackitem.Item {
	var result []stackitem.Item
	arr := item.Value().(iterator)
	for maxNum > 0 && arr.Next() {
		value, _ := arr.Value()
		result = append(result, value)
		maxNum--
	}
	return result
}
