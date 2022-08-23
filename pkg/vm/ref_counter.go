package vm

import (
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// refCounter represents a reference counter for the VM.
type refCounter int

func newRefCounter() *refCounter {
	return new(refCounter)
}

// Add adds an item to the reference counter.
func (r *refCounter) Add(item stackitem.Item) {
	if r == nil {
		return
	}
	*r++

	switch t := item.(type) {
	case *stackitem.Array:
		if t.IncRC() == 1 {
			for _, it := range t.Value().([]stackitem.Item) {
				r.Add(it)
			}
		}
	case *stackitem.Struct:
		if t.IncRC() == 1 {
			for _, it := range t.Value().([]stackitem.Item) {
				r.Add(it)
			}
		}
	case *stackitem.Map:
		if t.IncRC() == 1 {
			elems := t.Value().([]stackitem.MapElement)
			for i := range elems {
				r.Add(elems[i].Key)
				r.Add(elems[i].Value)
			}
		}
	}
}

// Remove removes an item from the reference counter.
func (r *refCounter) Remove(item stackitem.Item) {
	if r == nil {
		return
	}
	*r--

	switch t := item.(type) {
	case *stackitem.Array:
		if t.DecRC() == 0 {
			for _, it := range t.Value().([]stackitem.Item) {
				r.Remove(it)
			}
		}
	case *stackitem.Struct:
		if t.DecRC() == 0 {
			for _, it := range t.Value().([]stackitem.Item) {
				r.Remove(it)
			}
		}
	case *stackitem.Map:
		if t.DecRC() == 0 {
			elems := t.Value().([]stackitem.MapElement)
			for i := range elems {
				r.Remove(elems[i].Key)
				r.Remove(elems[i].Value)
			}
		}
	}
}
