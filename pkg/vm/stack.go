package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
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
// which will hold the underlying StackItem.
type Element struct {
	value      StackItem
	next, prev *Element
	stack      *Stack
}

// NewElement returns a new Element object, with its underlying value inferred
// to the corresponding type.
func NewElement(v interface{}) *Element {
	return &Element{
		value: makeStackItem(v),
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

// Item returns StackItem contained in the element.
func (e *Element) Item() StackItem {
	return e.value
}

// Value returns value of the StackItem contained in the element.
func (e *Element) Value() interface{} {
	return e.value.Value()
}

// BigInt attempts to get the underlying value of the element as a big integer.
// Will panic if the assertion failed which will be caught by the VM.
func (e *Element) BigInt() *big.Int {
	switch t := e.value.(type) {
	case *BigIntegerItem:
		return t.value
	case *BoolItem:
		if t.value {
			return big.NewInt(1)
		}
		return big.NewInt(0)
	default:
		b := t.Value().([]uint8)
		return emit.BytesToInt(b)
	}
}

// TryBool attempts to get the underlying value of the element as a boolean.
// Returns error if can't convert value to boolean type.
func (e *Element) TryBool() (bool, error) {
	switch t := e.value.(type) {
	case *BigIntegerItem:
		return t.value.Int64() != 0, nil
	case *BoolItem:
		return t.value, nil
	case *ArrayItem, *StructItem:
		return true, nil
	case *ByteArrayItem:
		for _, b := range t.value {
			if b != 0 {
				return true, nil
			}
		}
		return false, nil
	case *InteropItem:
		return t.value != nil, nil
	default:
		return false, fmt.Errorf("can't convert to bool: " + t.String())
	}
}

// Bool attempts to get the underlying value of the element as a boolean.
// Will panic if the assertion failed which will be caught by the VM.
func (e *Element) Bool() bool {
	val, err := e.TryBool()
	if err != nil {
		panic(err)
	}
	return val
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

// Array attempts to get the underlying value of the element as an array of
// other items. Will panic if the item type is different which will be caught
// by the VM.
func (e *Element) Array() []StackItem {
	switch t := e.value.(type) {
	case *ArrayItem:
		return t.value
	case *StructItem:
		return t.value
	default:
		panic("element is not an array")
	}
}

// Interop attempts to get the underlying value of the element
// as an interop item.
func (e *Element) Interop() *InteropItem {
	switch t := e.value.(type) {
	case *InteropItem:
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

	itemCount map[StackItem]int
	size      *int
}

// NewStack returns a new stack name by the given name.
func NewStack(n string) *Stack {
	s := &Stack{
		name: n,
	}
	s.top.next = &s.top
	s.top.prev = &s.top
	s.len = 0
	s.itemCount = make(map[StackItem]int)
	s.size = new(int)
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

	s.updateSizeAdd(e.value)

	return e
}

func (s *Stack) updateSizeAdd(item StackItem) {
	*s.size++

	switch item.(type) {
	case *ArrayItem, *StructItem, *MapItem:
		if s.itemCount[item]++; s.itemCount[item] > 1 {
			return
		}

		switch t := item.(type) {
		case *ArrayItem, *StructItem:
			for _, it := range item.Value().([]StackItem) {
				s.updateSizeAdd(it)
			}
		case *MapItem:
			for i := range t.value {
				s.updateSizeAdd(t.value[i].Value)
			}
		}
	}
}

func (s *Stack) updateSizeRemove(item StackItem) {
	*s.size--

	switch item.(type) {
	case *ArrayItem, *StructItem, *MapItem:
		if s.itemCount[item] > 1 {
			s.itemCount[item]--
			return
		}

		delete(s.itemCount, item)

		switch t := item.(type) {
		case *ArrayItem, *StructItem:
			for _, it := range item.Value().([]StackItem) {
				s.updateSizeRemove(it)
			}
		case *MapItem:
			for i := range t.value {
				s.updateSizeRemove(t.value[i].Value)
			}
		}
	}
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
// underlying StackItem to its corresponding type.
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

	s.updateSizeRemove(e.value)

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
	a := s.Peek(n1)
	b := s.Peek(n2)
	a.value, b.value = b.value, a.value
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
	case *ArrayItem:
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

// ToContractParameters converts Stack to slice of smartcontract.Parameter.
func (s *Stack) ToContractParameters() []smartcontract.Parameter {
	items := make([]smartcontract.Parameter, 0, s.Len())
	s.IterBack(func(e *Element) {
		// Each item is independent.
		seen := make(map[StackItem]bool)
		items = append(items, e.value.ToContractParameter(seen))
	})
	return items
}

// MarshalJSON implements JSON marshalling interface.
func (s *Stack) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.ToContractParameters())
}
