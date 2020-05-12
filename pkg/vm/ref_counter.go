package vm

// refCounter represents reference counter for the VM.
type refCounter struct {
	items map[StackItem]int
	size  int
}

func newRefCounter() *refCounter {
	return &refCounter{
		items: make(map[StackItem]int),
	}
}

// Add adds an item to the reference counter.
func (r *refCounter) Add(item StackItem) {
	r.size++

	switch item.(type) {
	case *ArrayItem, *StructItem, *MapItem:
		if r.items[item]++; r.items[item] > 1 {
			return
		}

		switch t := item.(type) {
		case *ArrayItem, *StructItem:
			for _, it := range item.Value().([]StackItem) {
				r.Add(it)
			}
		case *MapItem:
			for i := range t.value {
				r.Add(t.value[i].Value)
			}
		}
	}
}

// Remove removes item from the reference counter.
func (r *refCounter) Remove(item StackItem) {
	r.size--

	switch item.(type) {
	case *ArrayItem, *StructItem, *MapItem:
		if r.items[item] > 1 {
			r.items[item]--
			return
		}

		delete(r.items, item)

		switch t := item.(type) {
		case *ArrayItem, *StructItem:
			for _, it := range item.Value().([]StackItem) {
				r.Remove(it)
			}
		case *MapItem:
			for i := range t.value {
				r.Remove(t.value[i].Value)
			}
		}
	}
}
