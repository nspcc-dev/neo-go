package vm

// Slot is a fixed-size slice of stack items.
type Slot struct {
	storage []StackItem
}

// newSlot returns new slot of n items.
func newSlot(n int) *Slot {
	return &Slot{
		storage: make([]StackItem, n),
	}
}

// Set sets i-th storage slot.
func (s *Slot) Set(i int, item StackItem) {
	s.storage[i] = item
}

// Get returns item contained in i-th slot.
func (s *Slot) Get(i int) StackItem {
	if item := s.storage[i]; item != nil {
		return item
	}
	return NullItem{}
}

// Size returns slot size.
func (s *Slot) Size() int { return len(s.storage) }
