package vm

import (
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// refCounter represents reference counter for the VM.
type refCounter struct {
	items map[stackitem.Item]int
	size  int
}

func newRefCounter() *refCounter {
	return &refCounter{
		items: make(map[stackitem.Item]int),
	}
}

// Add adds an item to the reference counter.
func (r *refCounter) Add(item stackitem.Item) {
	if r == nil {
		return
	}
	r.size++

	switch item.(type) {
	case *stackitem.Array, *stackitem.Struct, *stackitem.Map:
		if r.items[item]++; r.items[item] > 1 {
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
}

// Remove removes item from the reference counter.
func (r *refCounter) Remove(item stackitem.Item) {
	if r == nil {
		return
	}
	r.size--

	switch item.(type) {
	case *stackitem.Array, *stackitem.Struct, *stackitem.Map:
		if r.items[item] > 1 {
			r.items[item]--
			return
		}

		delete(r.items, item)

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
}
