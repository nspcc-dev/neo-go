package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Stack implementation for the neo-go virtual machine. The stack implements
// a double linked list where its semantics are first in first out.
// To simplify the implementation, internally a Stack s is implemented as a
// ring, such that &s.top is both the next element of the last element s.Back()
// and the previous element of the first element s.Top().
//
// s.Push(0)
// s.Push(1)
// s.Push(2)
//
// [ 2 ] > top
// [ 1 ]
// [ 0 ] > back
//
// s.Pop() > 2
//
// [ 1 ]
// [ 0 ]

// Element represents an element in the double linked list (the stack),
// which will hold the underlying stackitem.Item.
type Element struct {
	value      stackitem.Item
	next, prev *Element
	stack      *Stack
}

// NewElement returns a new Element object, with its underlying value inferred
// to the corresponding type.
func NewElement(v interface{}) *Element {
	return &Element{
		value: stackitem.Make(v),
	}
}

// Next returns the next element in the stack.
func (e *Element) Next() *Element {
	if elem := e.next; e.stack != nil && elem != &e.stack.top {
		return elem
	}
	return nil
}

// Prev returns the previous element in the stack.
func (e *Element) Prev() *Element {
	if elem := e.prev; e.stack != nil && elem != &e.stack.top {
		return elem
	}
	return nil
}

// Item returns Item contained in the element.
func (e *Element) Item() stackitem.Item {
	return e.value
}

// Value returns value of the Item contained in the element.
func (e *Element) Value() interface{} {
	return e.value.Value()
}

// BigInt attempts to get the underlying value of the element as a big integer.
// Will panic if the assertion failed which will be caught by the VM.
func (e *Element) BigInt() *big.Int {
	val, err := e.value.TryInteger()
	if err != nil {
		panic(err)
	}
	return val
}

// Bool converts an underlying value of the element to a boolean if it's
// possible to do so, it will panic otherwise.
func (e *Element) Bool() bool {
	b, err := e.value.TryBool()
	if err != nil {
		panic(err)
	}
	return b
}

// Bytes attempts to get the underlying value of the element as a byte array.
// Will panic if the assertion failed which will be caught by the VM.
func (e *Element) Bytes() []byte {
	bs, err := e.value.TryBytes()
	if err != nil {
		panic(err)
	}
	return bs
}

// BytesOrNil attempts to get the underlying value of the element as a byte array or nil.
// Will panic if the assertion failed which will be caught by the VM.
func (e *Element) BytesOrNil() []byte {
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
func (e *Element) String() string {
	s, err := stackitem.ToString(e.value)
	if err != nil {
		panic(err)
	}
	return s
}

// Array attempts to get the underlying value of the element as an array of
// other items. Will panic if the item type is different which will be caught
// by the VM.
func (e *Element) Array() []stackitem.Item {
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
func (e *Element) Interop() *stackitem.Interop {
	switch t := e.value.(type) {
	case *stackitem.Interop:
		return t
	default:
		panic("element is not an interop")
	}
}

// Stack represents a Stack backed by a double linked list.
type Stack struct {
	top  Element
	name string
	len  int
	refs *refCounter
}

// NewStack returns a new stack name by the given name.
func NewStack(n string) *Stack {
	return newStack(n, newRefCounter())
}

func newStack(n string, refc *refCounter) *Stack {
	s := &Stack{
		name: n,
		refs: refc,
	}
	s.top.next = &s.top
	s.top.prev = &s.top
	return s
}

// Clear clears all elements on the stack and set its length to 0.
func (s *Stack) Clear() {
	s.top.next = &s.top
	s.top.prev = &s.top
	s.len = 0
}

// Len returns the number of elements that are on the stack.
func (s *Stack) Len() int {
	return s.len
}

// insert inserts the element after element (at) on the stack.
func (s *Stack) insert(e, at *Element) *Element {
	// If we insert an element that is already popped from this stack,
	// we need to clean it up, there are still pointers referencing to it.
	if e.stack == s {
		e = NewElement(e.value)
	}

	n := at.next
	at.next = e
	e.prev = at
	e.next = n
	n.prev = e
	e.stack = s
	s.len++

	s.refs.Add(e.value)

	return e
}

// InsertAt inserts the given item (n) deep on the stack.
// Be very careful using it and _always_ check both e and n before invocation
// as it will silently do wrong things otherwise.
func (s *Stack) InsertAt(e *Element, n int) *Element {
	before := s.Peek(n - 1)
	if before == nil {
		return nil
	}
	return s.insert(e, before)
}

// Push pushes the given element on the stack.
func (s *Stack) Push(e *Element) {
	s.insert(e, &s.top)
}

// PushVal pushes the given value on the stack. It will infer the
// underlying Item to its corresponding type.
func (s *Stack) PushVal(v interface{}) {
	s.Push(NewElement(v))
}

// Pop removes and returns the element on top of the stack.
func (s *Stack) Pop() *Element {
	return s.Remove(s.Top())
}

// Top returns the element on top of the stack. Nil if the stack
// is empty.
func (s *Stack) Top() *Element {
	if s.len == 0 {
		return nil
	}
	return s.top.next
}

// Back returns the element at the end of the stack. Nil if the stack
// is empty.
func (s *Stack) Back() *Element {
	if s.len == 0 {
		return nil
	}
	return s.top.prev
}

// Peek returns the element (n) far in the stack beginning from
// the top of the stack.
// 	n = 0 => will return the element on top of the stack.
func (s *Stack) Peek(n int) *Element {
	i := 0
	for e := s.Top(); e != nil; e = e.Next() {
		if n == i {
			return e
		}
		i++
	}
	return nil
}

// RemoveAt removes the element (n) deep on the stack beginning
// from the top of the stack.
func (s *Stack) RemoveAt(n int) *Element {
	return s.Remove(s.Peek(n))
}

// Remove removes and returns the given element from the stack.
func (s *Stack) Remove(e *Element) *Element {
	if e == nil {
		return nil
	}
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil // avoid memory leaks.
	e.prev = nil // avoid memory leaks.
	e.stack = nil
	s.len--

	s.refs.Remove(e.value)

	return e
}

// Dup duplicates and returns the element at position n.
// Dup is used for copying elements on to the top of its own stack.
// 	s.Push(s.Peek(0)) // will result in unexpected behaviour.
// 	s.Push(s.Dup(0)) // is the correct approach.
func (s *Stack) Dup(n int) *Element {
	e := s.Peek(n)
	if e == nil {
		return nil
	}

	return &Element{
		value: e.value.Dup(),
	}
}

// Iter iterates over all the elements int the stack, starting from the top
// of the stack.
// 	s.Iter(func(elem *Element) {
//		// do something with the element.
// 	})
func (s *Stack) Iter(f func(*Element)) {
	for e := s.Top(); e != nil; e = e.Next() {
		f(e)
	}
}

// IterBack iterates over all the elements of the stack, starting from the bottom
// of the stack.
// 	s.IterBack(func(elem *Element) {
//		// do something with the element.
// 	})
func (s *Stack) IterBack(f func(*Element)) {
	for e := s.Back(); e != nil; e = e.Prev() {
		f(e)
	}
}

// Swap swaps two elements on the stack without popping and pushing them.
func (s *Stack) Swap(n1, n2 int) error {
	if n1 < 0 || n2 < 0 {
		return errors.New("negative index")
	}
	if n1 >= s.len || n2 >= s.len {
		return errors.New("too big index")
	}
	if n1 == n2 {
		return nil
	}
	s.swap(n1, n2)
	return nil
}

func (s *Stack) swap(n1, n2 int) {
	a := s.Peek(n1)
	b := s.Peek(n2)
	a.value, b.value = b.value, a.value
}

// ReverseTop reverses top n items of the stack.
func (s *Stack) ReverseTop(n int) error {
	if n < 0 {
		return errors.New("negative index")
	} else if n > s.len {
		return errors.New("too big index")
	} else if n <= 1 {
		return nil
	}

	a, b := s.Peek(0), s.Peek(n-1)
	for i := 0; i < n/2; i++ {
		a.value, b.value = b.value, a.value
		a = a.Next()
		b = b.Prev()
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
	if n >= s.len {
		return errors.New("too big index")
	}
	if n == 0 {
		return nil
	}
	top := s.Peek(0)
	e := s.Peek(n)

	e.prev.next = e.next
	e.next.prev = e.prev

	top.prev = e
	e.next = top

	e.prev = &s.top
	s.top.next = e

	return nil
}

// PopSigElements pops keys or signatures from the stack as needed for
// CHECKMULTISIG.
func (s *Stack) PopSigElements() ([][]byte, error) {
	var num int
	var elems [][]byte
	item := s.Pop()
	if item == nil {
		return nil, fmt.Errorf("nothing on the stack")
	}
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
	items := make([]stackitem.Item, 0, s.len)
	s.IterBack(func(e *Element) {
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
