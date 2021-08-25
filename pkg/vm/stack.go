package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Stack implementation for the neo-go virtual machine. The stack with its LIFO
// semantics is emulated from simple slice where the top of the stack corresponds
// to the latest element of this slice. Pushes are appends to this slice, pops are
// slice resizes.

// Element represents an element on the stack, technically it's a wrapper around
// stackitem.Item interface to provide some API simplification for VM.
type Element struct {
	value stackitem.Item
}

// NewElement returns a new Element object, with its underlying value inferred
// to the corresponding type.
func NewElement(v interface{}) Element {
	return Element{stackitem.Make(v)}
}

// Item returns Item contained in the element.
func (e Element) Item() stackitem.Item {
	return e.value
}

// Value returns value of the Item contained in the element.
func (e Element) Value() interface{} {
	return e.value.Value()
}

// BigInt attempts to get the underlying value of the element as a big integer.
// Will panic if the assertion failed which will be caught by the VM.
func (e Element) BigInt() *big.Int {
	val, err := e.value.TryInteger()
	if err != nil {
		panic(err)
	}
	return val
}

// Bool converts an underlying value of the element to a boolean if it's
// possible to do so, it will panic otherwise.
func (e Element) Bool() bool {
	b, err := e.value.TryBool()
	if err != nil {
		panic(err)
	}
	return b
}

// Bytes attempts to get the underlying value of the element as a byte array.
// Will panic if the assertion failed which will be caught by the VM.
func (e Element) Bytes() []byte {
	bs, err := e.value.TryBytes()
	if err != nil {
		panic(err)
	}
	return bs
}

// BytesOrNil attempts to get the underlying value of the element as a byte array or nil.
// Will panic if the assertion failed which will be caught by the VM.
func (e Element) BytesOrNil() []byte {
	if _, ok := e.value.(stackitem.Null); ok {
		return nil
	}
	bs, err := e.value.TryBytes()
	if err != nil {
		panic(err)
	}
	return bs
}

// String attempts to get string from the element value.
// It is assumed to be use in interops and panics if string is not a valid UTF-8 byte sequence.
func (e Element) String() string {
	s, err := stackitem.ToString(e.value)
	if err != nil {
		panic(err)
	}
	return s
}

// Array attempts to get the underlying value of the element as an array of
// other items. Will panic if the item type is different which will be caught
// by the VM.
func (e Element) Array() []stackitem.Item {
	switch t := e.value.(type) {
	case *stackitem.Array:
		return t.Value().([]stackitem.Item)
	case *stackitem.Struct:
		return t.Value().([]stackitem.Item)
	default:
		panic("element is not an array")
	}
}

// Interop attempts to get the underlying value of the element
// as an interop item.
func (e Element) Interop() *stackitem.Interop {
	switch t := e.value.(type) {
	case *stackitem.Interop:
		return t
	default:
		panic("element is not an interop")
	}
}

// Stack represents a Stack backed by a slice of Elements.
type Stack struct {
	elems []Element
	name  string
	refs  *refCounter
}

// NewStack returns a new stack name by the given name.
func NewStack(n string) *Stack {
	return newStack(n, newRefCounter())
}

func newStack(n string, refc *refCounter) *Stack {
	s := new(Stack)
	s.elems = make([]Element, 0, 16) // Most of uses are expected to fit into 16 elements.
	initStack(s, n, refc)
	return s
}
func initStack(s *Stack, n string, refc *refCounter) {
	s.name = n
	s.refs = refc
	s.Clear()
}

// Clear clears all elements on the stack and set its length to 0.
func (s *Stack) Clear() {
	if s.elems != nil {
		for _, el := range s.elems {
			s.refs.Remove(el.value)
		}
		s.elems = s.elems[:0]
	}
}

// Len returns the number of elements that are on the stack.
func (s *Stack) Len() int {
	return len(s.elems)
}

// InsertAt inserts the given item (n) deep on the stack.
// Be very careful using it and _always_ check n before invocation
// as it will panic otherwise.
func (s *Stack) InsertAt(e Element, n int) {
	l := len(s.elems)
	s.elems = append(s.elems, e)
	copy(s.elems[l-n+1:], s.elems[l-n:l])
	s.elems[l-n] = e
	s.refs.Add(e.value)
}

// Push pushes the given element on the stack.
func (s *Stack) Push(e Element) {
	s.elems = append(s.elems, e)
	s.refs.Add(e.value)
}

// PushVal pushes the given value on the stack. It will infer the
// underlying Item to its corresponding type.
func (s *Stack) PushVal(v interface{}) {
	s.Push(NewElement(v))
}

// Pop removes and returns the element on top of the stack. Panics if stack is
// empty.
func (s *Stack) Pop() Element {
	l := len(s.elems)
	e := s.elems[l-1]
	s.elems = s.elems[:l-1]
	s.refs.Remove(e.value)
	return e
}

// Top returns the element on top of the stack. Nil if the stack
// is empty.
func (s *Stack) Top() Element {
	if len(s.elems) == 0 {
		return Element{}
	}
	return s.elems[len(s.elems)-1]
}

// Back returns the element at the end of the stack. Nil if the stack
// is empty.
func (s *Stack) Back() Element {
	if len(s.elems) == 0 {
		return Element{}
	}
	return s.elems[0]
}

// Peek returns the element (n) far in the stack beginning from
// the top of the stack. For n == 0 it's effectively the same as Top,
// but it'll panic if the stack is empty.
func (s *Stack) Peek(n int) Element {
	n = len(s.elems) - n - 1
	return s.elems[n]
}

// RemoveAt removes the element (n) deep on the stack beginning
// from the top of the stack. Panics if called with out of bounds n.
func (s *Stack) RemoveAt(n int) Element {
	l := len(s.elems)
	e := s.elems[l-1-n]
	s.elems = append(s.elems[:l-1-n], s.elems[l-n:]...)
	s.refs.Remove(e.value)
	return e
}

// Dup duplicates and returns the element at position n.
// Dup is used for copying elements on to the top of its own stack.
// 	s.Push(s.Peek(0)) // will result in unexpected behaviour.
// 	s.Push(s.Dup(0)) // is the correct approach.
func (s *Stack) Dup(n int) Element {
	e := s.Peek(n)
	return Element{e.value.Dup()}
}

// Iter iterates over all the elements int the stack, starting from the top
// of the stack.
// 	s.Iter(func(elem *Element) {
//		// do something with the element.
// 	})
func (s *Stack) Iter(f func(Element)) {
	for i := len(s.elems) - 1; i >= 0; i-- {
		f(s.elems[i])
	}
}

// IterBack iterates over all the elements of the stack, starting from the bottom
// of the stack.
// 	s.IterBack(func(elem *Element) {
//		// do something with the element.
// 	})
func (s *Stack) IterBack(f func(Element)) {
	for i := 0; i < len(s.elems); i++ {
		f(s.elems[i])
	}
}

// Swap swaps two elements on the stack without popping and pushing them.
func (s *Stack) Swap(n1, n2 int) error {
	if n1 < 0 || n2 < 0 {
		return errors.New("negative index")
	}
	l := len(s.elems)
	if n1 >= l || n2 >= l {
		return errors.New("too big index")
	}
	s.elems[l-n1-1], s.elems[l-n2-1] = s.elems[l-n2-1], s.elems[l-n1-1]
	return nil
}

// ReverseTop reverses top n items of the stack.
func (s *Stack) ReverseTop(n int) error {
	l := len(s.elems)
	if n < 0 {
		return errors.New("negative index")
	} else if n > l {
		return errors.New("too big index")
	} else if n <= 1 {
		return nil
	}

	for i, j := l-n, l-1; i <= j; i, j = i+1, j-1 {
		s.elems[i], s.elems[j] = s.elems[j], s.elems[i]
	}
	return nil
}

// Roll brings an item with the given index to the top of the stack, moving all
// the other elements down accordingly. It does all of that without popping and
// pushing elements.
func (s *Stack) Roll(n int) error {
	if n < 0 {
		return errors.New("negative index")
	}
	l := len(s.elems)
	if n >= l {
		return errors.New("too big index")
	}
	if n == 0 {
		return nil
	}
	e := s.elems[l-1-n]
	copy(s.elems[l-1-n:], s.elems[l-n:])
	s.elems[l-1] = e
	return nil
}

// PopSigElements pops keys or signatures from the stack as needed for
// CHECKMULTISIG.
func (s *Stack) PopSigElements() ([][]byte, error) {
	var num int
	var elems [][]byte
	if s.Len() == 0 {
		return nil, fmt.Errorf("nothing on the stack")
	}
	item := s.Pop()
	switch item.value.(type) {
	case *stackitem.Array:
		num = len(item.Array())
		if num < 1 {
			return nil, fmt.Errorf("less than one element in the array")
		}
		elems = make([][]byte, num)
		for k, v := range item.Array() {
			b, ok := v.Value().([]byte)
			if !ok {
				return nil, fmt.Errorf("bad element %s", v.String())
			}
			elems[k] = b
		}
	default:
		num = int(item.BigInt().Int64())
		if num < 1 || num > s.Len() {
			return nil, fmt.Errorf("wrong number of elements: %d", num)
		}
		elems = make([][]byte, num)
		for i := 0; i < num; i++ {
			elems[i] = s.Pop().Bytes()
		}
	}
	return elems, nil
}

// ToArray converts stack to an array of stackitems with top item being the last.
func (s *Stack) ToArray() []stackitem.Item {
	items := make([]stackitem.Item, 0, len(s.elems))
	s.IterBack(func(e Element) {
		items = append(items, e.Item())
	})
	return items
}

// MarshalJSON implements JSON marshalling interface.
func (s *Stack) MarshalJSON() ([]byte, error) {
	items := s.ToArray()
	arr := make([]json.RawMessage, len(items))
	for i := range items {
		data, err := stackitem.ToJSONWithTypes(items[i])
		if err == nil {
			arr[i] = data
		}
	}
	return json.Marshal(arr)
}
