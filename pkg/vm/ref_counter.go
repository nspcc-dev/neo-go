package vm

import (
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// refCounter represents a reference counter for the VM.
type refCounter int

type (
	rcInc interface {
		IncRC() int
	}
	rcDec interface {
		DecRC() int
	}
)

func newRefCounter() *refCounter {
	return new(refCounter)
}

// Add adds an item to the reference counter.
func (r *refCounter) Add(item stackitem.Item) {
	if r == nil {
		return
	}
	*r++

	irc, ok := item.(rcInc)
	if !ok || irc.IncRC() > 1 {
		return
	}
	switch t := item.(type) {
	case *stackitem.Array, *stackitem.Struct:
		for _, it := range item.Value().([]stackitem.Item) {
			r.Add(it)
		}
	case *stackitem.Map:
		elems := t.Value().([]stackitem.MapElement)
		for i := range elems {
			r.Add(elems[i].Key)
			r.Add(elems[i].Value)
		}
	}
}

// Remove removes an item from the reference counter.
func (r *refCounter) Remove(item stackitem.Item) {
	if r == nil {
		return
	}
	*r--

	irc, ok := item.(rcDec)
	if !ok || irc.DecRC() > 0 {
		return
	}
	switch t := item.(type) {
	case *stackitem.Array, *stackitem.Struct:
		for _, it := range item.Value().([]stackitem.Item) {
			r.Remove(it)
		}
	case *stackitem.Map:
		elems := t.Value().([]stackitem.MapElement)
		for i := range elems {
			r.Remove(elems[i].Key)
			r.Remove(elems[i].Value)
		}
	}
}
