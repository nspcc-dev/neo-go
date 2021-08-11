package vm

import (
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// refCounter represents reference counter for the VM.
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
		for i := range t.Value().([]stackitem.MapElement) {
			r.Add(t.Value().([]stackitem.MapElement)[i].Value)
		}
	}
}

// Remove removes item from the reference counter.
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
		for i := range t.Value().([]stackitem.MapElement) {
			r.Remove(t.Value().([]stackitem.MapElement)[i].Value)
		}
	}
}
